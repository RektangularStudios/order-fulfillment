package products

import (
	"context"
	"math/rand"
	"time"
	"fmt"

	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
)

const (
	RARE = "RARE"
	KINDA_RARE = "KINDA_RARE"
	NOT_THAT_RARE = "NOT_THAT_RARE"
)

func OccultaNovelliaRare() []string {
	return []string{
		"PROD-01F4MK4ZCVTKAAZF1QZAPWMPFP",
		"PROD-01F4MK4ZNC8FMVR2ANHDW9E1N4",
		"PROD-01F4MK4ZYC6P9EGG4W0DNFQTWS",
	}
}

func OccultaNovelliaKindaRare() []string {
	return []string{
		"PROD-01F4MK45QJS4WZ1VBZW1A1THD7",
		"PROD-01F4MK4NTCXGVA35CAD7TCHEM8",
		"PROD-01F4MK4P5SMNGKBF5B7AKN35YD",
		"PROD-01F4MK4PF52A72Y7P77TEPA2CW",
		"PROD-01F4MK4PRD20D3Z95T84ZYA0SX",
	}
}

func OccultaNovelliaNotThatRare() []string {
	return []string{
		"PROD-01F4MK4XRGJV2NR9XNQY9GCPGQ",
		"PROD-01F4MK4Y26J6A66YQ1PXH8NXMC",
		"PROD-01F4MK4YAR07BTRSQFHDWNXC55",
		"PROD-01F4MK4YKAJ0REHHDY63TTTEWM",
		"PROD-01F4MK4YVW4JSV717E0XK920AZ",
		"PROD-01F4MK4Z489EBKGGFXA2HKZ1MA",
	}
}

type ServiceImpl struct {
	novelliaDatabaseService novellia_database.Service
	products map[string]novellia_database.Product
	randSource rand.Source
}

// creates a new ServiceImpl
func New(novelliaDatabaseService novellia_database.Service) *ServiceImpl {
	randSource := rand.NewSource(time.Now().Unix())

	return &ServiceImpl {
		novelliaDatabaseService: novelliaDatabaseService,
		randSource: randSource,
	}
}

func (s *ServiceImpl) GetProducts(ctx context.Context) (map[string]novellia_database.Product, error) {
	// fetch and cache products
	if len(s.products) == 0 {
		products, err := s.novelliaDatabaseService.QueryProducts(ctx)
		if err != nil {
			return nil, err
		}

		m := make(map[string]novellia_database.Product)
		for _, v := range products {
			m[v.ProductID] = v
		}
		s.products = m
	}

	return s.products, nil
}

func (s *ServiceImpl) GetBoosterCard() string {
	r := rand.New(s.randSource)

	var t string
	p := r.Intn(100)
	if p < 1 { // 0 / [0,99]
		t = RARE
	} else if p < 25 { // 1-24 / [0,99]
		t = KINDA_RARE
	} else { // 25-99 / [0,99]
		t = NOT_THAT_RARE
	}

	switch t {
	case RARE:
		cardIndex := r.Intn(len(OccultaNovelliaRare()))
		return OccultaNovelliaRare()[cardIndex]
	case KINDA_RARE:
		cardIndex := r.Intn(len(OccultaNovelliaKindaRare()))
		return OccultaNovelliaKindaRare()[cardIndex]
	default: // NOT_THAT_RARE
		cardIndex := r.Intn(len(OccultaNovelliaNotThatRare()))
		return OccultaNovelliaNotThatRare()[cardIndex]
	}
}

// converts a product ID representing a bundle into a list of atomic product IDs
func (s *ServiceImpl) UnpackBundleProduct(productID string) ([]string, error) {
	// initialize local pseudorandom generator 
	r := rand.New(s.randSource)

	unpackedProducts := []string{}
	switch productID {
	// Starter Deck
	case "PROD-01F4NAFJCAG5JDEGMR0XQARBW2":
		rareIndex := r.Intn(len(OccultaNovelliaRare()))
		unpackedProducts = append(unpackedProducts, OccultaNovelliaRare()[rareIndex])
		unpackedProducts = append(unpackedProducts, OccultaNovelliaKindaRare()...)
		unpackedProducts = append(unpackedProducts, OccultaNovelliaNotThatRare()...)
		if len(unpackedProducts) != 12 {
			return nil, fmt.Errorf("starter deck must have 12 cards, got %+v", unpackedProducts)
		}
		return unpackedProducts, nil
	// Booster Pack
	case "PROD-01F4NAF8MANXDT26MGA5E0QXNJ":
		for i := 0; i < 3; i++ {
			t := s.GetBoosterCard()
			unpackedProducts = append(unpackedProducts, t)
		}
		if len(unpackedProducts) != 3 {
			return nil, fmt.Errorf("booster pack must have 3 cards, got %+v", unpackedProducts)
		}
		return unpackedProducts, nil
	default:
		return []string{productID}, nil
	}
}
