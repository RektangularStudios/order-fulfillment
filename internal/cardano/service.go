package cardano

// IMPORTANT: cardano-cli commands were written for version 1.25.1

import (
	"context"
	"fmt"
	"os/exec"
	"encoding/json"
	"path/filepath"
	"io/ioutil"
	"math/big"
	"strings"
	"bytes"

	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/products"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/constants"
	prometheus_monitoring "bitbucket.org/ConcurrentDragon/order-fulfillment/internal/monitoring"
	ordf "github.com/RektangularStudios/novellia-sdk/sdk/server/go/order_fulfillment/v0"
)

type Asset struct {
	CurrencyID string `json:"currency_id"`
	Quantity *big.Int `json:"quantity"`
}

type UTXO struct {
	TXID string `json:"txid"`
	Assets []Asset `json:"assets"`
}

type UTXOs struct {
	UTXOs []UTXO `json:"utxos"`
}

type ServiceImpl struct {
	novelliaDatabaseService novellia_database.Service
	productsService products.Service
	hotWalletSigningKeyPath string
	hotWalletAddress string
	scriptsPath string
	protocolParamsPath string
	usedUTXOs []string
}

// creates a new ServiceImpl
func New(novelliaDatabaseService novellia_database.Service, productsService products.Service) (*ServiceImpl, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get config from env")
	}

	return &ServiceImpl {
		novelliaDatabaseService: novelliaDatabaseService,
		productsService: productsService,
		hotWalletSigningKeyPath: cfg.Cardano.HotWalletSigningKeyPath,
		hotWalletAddress: cfg.Cardano.HotWalletAddress,
		scriptsPath: cfg.Cardano.ScriptsPath,
		protocolParamsPath: cfg.Cardano.ProtocolParamsPath,
		usedUTXOs: []string{},
	}, nil
}

func (s *ServiceImpl) NativeTokensFromOrder(ctx context.Context, order *ordf.Order) (map[string]*big.Int, error) {
		// get products list
		products, err := s.productsService.GetProducts(ctx)
		if err != nil {
			return nil, err
		}

		// generate native token list
		tokenQuantities := map[string]*big.Int{}
		for _, item := range order.Items {
			for i := 0; i < int(item.Quantity); i++ {
				// unpack bundle
				// we iterate for each quantity so that random functions for bundles produce different outcomes
				unpackedProductIDs, err := s.productsService.UnpackBundleProduct(item.ProductId)
				if err != nil {
					return nil, fmt.Errorf("failed to unpack productID %s: %+v", item.ProductId, err)
				}
	
				for j := 0; j < len(unpackedProductIDs); j++ {
					var nativeTokenID string
					if _, ok := products[unpackedProductIDs[j]]; ok {
						nativeTokenID = products[unpackedProductIDs[j]].NativeTokenID
					} else {
						return nil, fmt.Errorf("invalid product ID from unpack %s not found", unpackedProductIDs[j])
					}
					if _, ok := tokenQuantities[nativeTokenID]; !ok {
						tokenQuantities[nativeTokenID] = big.NewInt(0)
					}
					tokenQuantities[nativeTokenID].Add(tokenQuantities[nativeTokenID], big.NewInt(1))
				}
			}	
		}

		return tokenQuantities, nil
}

func (s *ServiceImpl) GetUTXOs(address string, filenameSalt string) (*UTXOs, error) {
	// dump UTXO list to file
	utxoJSONPath := fmt.Sprintf("utxos_%s.json", filenameSalt)
	_, err := exec.Command("cardano-cli", "query", "utxo", "--address", address, "--mainnet", "--mary-era", "--out-file", utxoJSONPath).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to dump UTXOs: %v", err)
	}

	// convert JSON to something sane
	utxoJSONNicePath := fmt.Sprintf("utxos_%s_nice.json", filenameSalt)
	_, err = exec.Command("python3", filepath.Join(s.scriptsPath, "parseUTXOs.py"), utxoJSONPath, utxoJSONNicePath).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to convert UTXOs JSON: %v", err)
	}

	// load nice JSON
	utxoJSONNiceBytes, err := ioutil.ReadFile(utxoJSONNicePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read nice JSON file %s: %v", utxoJSONNicePath, err)
	}

	var utxos UTXOs
	err = json.Unmarshal(utxoJSONNiceBytes, &utxos)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal nice UTXO JSON: %v", err)
	}

	return &utxos, nil
}

func (s *ServiceImpl) GetTTL() (*big.Int, error) {
	out, err := exec.Command("cardano-cli", "query", "tip", "--mainnet").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query cardano tip: %v", err)
	}

	type QueryTip struct {
		BlockNo *big.Int `json:"blockNo"`
		HeaderHash string `json:"headerHash"`
		SlotNo *big.Int `json:"slotNo"`
	}
	
	var res QueryTip
	err = json.Unmarshal(out, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal query tip JSON: %v", err)
	}

	ttlOffset := big.NewInt(constants.TTLOffset)
	return res.SlotNo.Add(res.SlotNo, ttlOffset), nil
}

// returns --tx-in and --tx-out string args
func (s *ServiceImpl) processUTXO(utxo UTXO, currentTokens map[string]*big.Int, goalTokens map[string]*big.Int) (string, string) {
	partiallySpent := false
	usedUTXO := false
	for i, asset := range utxo.Assets {
		// do not use lovelace already paired with native tokens
		if asset.CurrencyID == "lovelace" && len(utxo.Assets) != 1 {
			continue
		}

		// check if asset is needed
		if _, ok := goalTokens[asset.CurrencyID]; !ok {
			continue
		}

		// initialize current token sum
		if _, ok := currentTokens[asset.CurrencyID]; !ok {
			currentTokens[asset.CurrencyID] = big.NewInt(0)
		}

		// check if asset quantity has been met
		if goalTokens[asset.CurrencyID].Cmp(currentTokens[asset.CurrencyID]) == 0 {
			continue
		}

		amountToUse := big.NewInt(0).Sub(goalTokens[asset.CurrencyID], currentTokens[asset.CurrencyID])
		switch asset.Quantity.Cmp(amountToUse) {
		case -1:
			// cannot spend more than what is in UTXO
			amountToUse = asset.Quantity
		case 1:
			// UTXO is partially spent
			partiallySpent = true
		case 0:
			// exact
		}

		// subtract and add used quantity
		utxo.Assets[i].Quantity = big.NewInt(0).Sub(utxo.Assets[i].Quantity, amountToUse)
		currentTokens[asset.CurrencyID] = big.NewInt(0).Add(currentTokens[asset.CurrencyID], amountToUse)
		usedUTXO = true

		// ada must be leftover from spent native tokens
		if len(utxo.Assets) != 1 {
			partiallySpent = true
		}
	}

	// if UTXO is only partially spent, need a new --tx-out
	txOut := ""
	if partiallySpent {
		txOut = s.hotWalletAddress
		for _, asset := range utxo.Assets {
			// quantity > 0
			if asset.Quantity.Cmp(big.NewInt(0)) == 1 {
				txOut = fmt.Sprintf("%s + %d %s", txOut, asset.Quantity, asset.CurrencyID)
			}
		}
	}

	// spend the UTXO
	txIn := ""
	if usedUTXO {
		txIn = utxo.TXID
	}

	return txIn, txOut
}

func (s *ServiceImpl) WriteRawTX(deliveryAddress string, nativeTokens map[string]*big.Int, utxos *UTXOs, txRawPathOut string, feeLovelace *big.Int, ttl *big.Int) (int, int, error) {	
	// add min-ada, we will manually subtract the fee from this
	minLovelace := big.NewInt(constants.MinADA * 1000000)
	nativeTokens["lovelace"] = minLovelace

	currentTokensIn := map[string]*big.Int{}
	txInArgs := []string{}
	txOutArgs := []string{}

	// process UTXOs
	for _, utxo := range utxos.UTXOs {
		txIn, txOut := s.processUTXO(utxo, currentTokensIn, nativeTokens)
		if txIn != "" {
			txInArgs = append(txInArgs, txIn)
		}
		if txOut != "" {
			txOutArgs = append(txOutArgs, txOut)
		}
	}

	// verify input requirements were met, otherwise not enough UTXOs
	for currency_id, quantity := range nativeTokens {
		if currentTokensIn[currency_id].Cmp(quantity) == -1 {
			prometheus_monitoring.TickCardanoInsufficientUTXOs()
			return 0, 0, fmt.Errorf("insufficient UTXOs to satisfy %s %d requirement", currency_id, quantity)
		}
	}

	// create single txOut for order recipient
	minAdaLessFee := big.NewInt(0).Sub(minLovelace, feeLovelace)
	txOutDelivery := fmt.Sprintf("%s+%d", deliveryAddress, minAdaLessFee)
	for currency_id, quantity := range nativeTokens {
		if currency_id == "lovelace" {
			continue
		}
		txOutDelivery = fmt.Sprintf("%s + %d %s", txOutDelivery, quantity, currency_id,)
	}
	txOutArgs = append(txOutArgs, txOutDelivery)

	txArgs := []string{}
	for _, txIn := range txInArgs {
		txArgs = append(txArgs, "--tx-in")
		txArgs = append(txArgs, txIn)
	}
	for _, txOut := range txOutArgs {
		txArgs = append(txArgs, "--tx-out")
		txArgs = append(txArgs, txOut)
	}
	commandArgs := []string{
		"transaction",
		"build-raw",
	}
	otherArgs := []string{
		"--ttl", fmt.Sprintf("%d", ttl),
		"--fee", fmt.Sprintf("%d", feeLovelace),
		"--out-file", txRawPathOut,
		"--mary-era",
	}

	args := append(commandArgs, txArgs...)
	args = append(args, otherArgs...)
	cmd := exec.Command("cardano-cli", args...)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to write raw tx:" + fmt.Sprint(err) + ": " + stderr.String())
	}

	return len(txInArgs), len(txOutArgs), nil
}

func (s *ServiceImpl) GetFee(txRawPath string, txInCount, txOutCount int) (*big.Int, error) {
	out, err := exec.Command("cardano-cli", "transaction", "calculate-min-fee",
		"--tx-body-file", txRawPath,
		"--tx-in-count", fmt.Sprintf("%d", txInCount),
		"--tx-out-count", fmt.Sprintf("%d", txOutCount),
		"--witness-count", "1",
		"--mainnet",
		"--protocol-params-file", s.protocolParamsPath,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get minimum transaction fee: %v", err)
	}

	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return nil, fmt.Errorf("failed to parse fee string, expected 2 fields")
	}
	
	feeStr := fields[0]
	fee, ok := new(big.Int).SetString(feeStr, 10)
	if !ok {
		return nil, fmt.Errorf("failed to convert fee to big int (%s), bytes from command: %s", feeStr, string(out))
	}

	return fee, nil
}

func (s *ServiceImpl) SignTX(txRawPath string, txSignedOutPath string) error {
	out, err := exec.Command("cardano-cli", "transaction", "sign",
		"--tx-body-file", txRawPath,
		"--signing-key-file", s.hotWalletSigningKeyPath,
		"--mainnet",
		"--out-file", txSignedOutPath,
	).Output()
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}
	if string(out) != "" {
		return fmt.Errorf("failed to sign transaction, cardano-cli returned: %s", string(out))
	}

	return nil
}

func (s *ServiceImpl) SubmitTX(txSignedPath string) error {
	cmd := exec.Command("cardano-cli", "transaction", "submit", "--tx-file", txSignedPath, "--mainnet")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to submit transaction (command failed):" + fmt.Sprint(err) + ": " + stderr.String())
	}
	return nil
}

func (s *ServiceImpl) GetTXID(txSignedPath string) (string, error) {
	out, err := exec.Command("cardano-cli", "transaction", "txid", "--tx-file", txSignedPath).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get TXID: %v", err)
	}

	return string(out), nil
}

func (s *ServiceImpl) SubmitOrder(ctx context.Context, order *ordf.Order) (string, error) {
	tokenQuantities, err := s.novelliaDatabaseService.QueryOrderNativeTokens(ctx, order.OrderId)
	if err != nil {
		return "", err
	}

	ttl, err := s.GetTTL()
	if err != nil {
		return "", err
	}

	utxos, err := s.GetUTXOs(s.hotWalletAddress, order.OrderId)
	if err != nil {
		return "", err
	}
	txRawPath := fmt.Sprintf("tx_%s.raw", order.OrderId)
	txInCount, txOutCount, err := s.WriteRawTX(order.Customer.DeliveryAddress, tokenQuantities, utxos, txRawPath, big.NewInt(0), ttl)
	if err != nil {
		return "", err
	}

	fee, err := s.GetFee(txRawPath, txInCount, txOutCount)
	if err != nil {
		return "", err
	}

	// get UTXOs again because WriteRawTX modified them, TODO: do deep copy
	utxos, err = s.GetUTXOs(s.hotWalletAddress, order.OrderId)
	if err != nil {
		return "", err
	}
	_, _, err = s.WriteRawTX(order.Customer.DeliveryAddress, tokenQuantities, utxos, txRawPath, fee, ttl)
	if err != nil {
		return "", err
	}

	txSignedPath := fmt.Sprintf("tx_%s.signed", order.OrderId)
	err = s.SignTX(txRawPath, txSignedPath)
	if err != nil {
		return "", err
	}

	// verify this order does not already have a TXID


	err = s.SubmitTX(txSignedPath)
	if err != nil {
		return "", err
	}
	prometheus_monitoring.TickSubmittedToCardano()

	txid, err := s.GetTXID(txSignedPath)
	if err != nil {
		return "", err
	}

	return txid, nil
}

func (s *ServiceImpl) ValidateAddress(address string) (error) {
	out, err := exec.Command("cardano-cli", "address", "info",
		"--address", address,
	).Output()
	if err != nil {
		return fmt.Errorf("failed to validate address (cmd): %v", err)
	}
	if strings.Contains(string(out), "Invalid") {
		return fmt.Errorf("address is invalid: %s, output: %s", address, string(out))
	}

	return nil
}

func (s *ServiceImpl) GetStock(addresses []string) (map[string]*big.Int, error) {
	tokens := map[string]*big.Int{}
	for _, address := range addresses {
		salt := s.novelliaDatabaseService.GenerateULID("GETSTOCK")
		utxos, err := s.GetUTXOs(address, salt)
		if err != nil {
			return nil, fmt.Errorf("failed to get stock: %+v", err)
		}
		for _, utxo := range utxos.UTXOs {
			for _, asset := range utxo.Assets {
				if _, ok := tokens[asset.CurrencyID]; !ok {
					tokens[asset.CurrencyID] = big.NewInt(0)
				}
				tokens[asset.CurrencyID] = big.NewInt(0).Add(tokens[asset.CurrencyID], asset.Quantity)
			}
		}
	}

	return tokens, nil
}

func (s *ServiceImpl) HotWalletAddress() string {
	return s.hotWalletAddress
}
