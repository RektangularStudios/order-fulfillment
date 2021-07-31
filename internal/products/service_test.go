package products_test

import (
	"fmt"
	"context"
	"testing"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/novellia_database"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/products"
	"bitbucket.org/ConcurrentDragon/order-fulfillment/internal/config"
)

const (
	configPath = "/config/prod-live.yaml"
)

func setupTest(ctx context.Context) (novellia_database.Service, error) {
	err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	novelliaDatabaseService, err := novellia_database.New(
		ctx,
		config.Postgres.Username,
		config.Postgres.Password,
		config.Postgres.Host,
		config.Postgres.Database,
		config.Postgres.QueriesPath,
	)
	if err != nil {
		return nil, err
	}

	return novelliaDatabaseService, nil
}

func TestGetProducts(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()
	productsService := products.New(novelliaDatabaseService)

	products, err := productsService.GetProducts(ctx)
	if err != nil {
		t.Errorf("get products failed: %+v", err)
	}
	fmt.Printf("%+v", products)
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
			if b == a {
					return true
			}
	}
	return false
}

func TestUnpackBundleProduct(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()
	productsService := products.New(novelliaDatabaseService)

	// test Starter Deck
	randStarterRare := map[string]int{}
	// iterate to test RNG
	for i := 0; i < 1000; i++ {
		unbundled, err := productsService.UnpackBundleProduct("PROD-01F4NAFJCAG5JDEGMR0XQARBW2")
		if err != nil {
			t.Errorf("failed to unbundle starter deck: %+v", err)
		}
		if len(unbundled) != 12 {
			t.Errorf("starter deck must have 12 cards")
		}

		// check that base 11 cards exist
		baseStarterDeckCards := append(products.OccultaNovelliaNotThatRare(), products.OccultaNovelliaKindaRare()...)
		for _, c := range baseStarterDeckCards {
			if !stringInSlice(c, unbundled) {
				t.Errorf("missing %s in unbundled starter deck: %+v", c, err)
			}
		}

		// tally distribution of rares
		for _, r := range products.OccultaNovelliaRare() {
			if stringInSlice(r, unbundled) {
				randStarterRare[r] += 1
				break
			}
		}
	}
	// validate tally of rares
	for _, r := range products.OccultaNovelliaRare() {
		if randStarterRare[r] < 300 {
			t.Errorf("low count of %s rare in unbundled starter deck", r)
		}
	}
	fmt.Printf("Starter rares distribution: %+v", randStarterRare)

	// test Booster Pack
	booster1, err := productsService.UnpackBundleProduct("PROD-01F4NAF8MANXDT26MGA5E0QXNJ")
	if err != nil {
		t.Errorf("failed to unbundle booster pack 1: %+v", err)
	}
	booster2, err := productsService.UnpackBundleProduct("PROD-01F4NAF8MANXDT26MGA5E0QXNJ")
	if err != nil {
		t.Errorf("failed to unbundle booster pack 2: %+v", err)
	}
	if len(booster1) != 3 || len(booster2) != 3 {
		t.Errorf("booster pack must have 3 cards")
	}
	// yes, this will occassionally fail
	differentBooster := false
	for i := range booster1 {
		if booster1[i] != booster2[i] {
			differentBooster = true
		}
	}
	if !differentBooster {
		t.Errorf("boosters should not be identical")
	}
}

func TestGetBoosterCard(t *testing.T) {
	ctx := context.Background()

	novelliaDatabaseService, err := setupTest(ctx)
	if err != nil {
		t.Errorf("failed to setup test: %+v", err)
	}
	defer novelliaDatabaseService.Close()
	productsService := products.New(novelliaDatabaseService)

	randCards := map[string]int{}
	cardIters := 1000000
	for i := 0; i < cardIters; i++ {
		c := productsService.GetBoosterCard()
		randCards[c] += 1
	}

	// validate card distribution
	if len(randCards) != 14 {
		t.Errorf("booster pack random distribution must have all 14 cards, got %d", len(randCards))
	}

	toleranceFactor := 0.05
	for _, c := range products.OccultaNovelliaRare() {
		target := float64(cardIters) * 0.01 / float64(len(products.OccultaNovelliaRare()))
		v := float64(randCards[c])
		lb := target * (1 - toleranceFactor)
		ub := target * (1 + toleranceFactor)
		if  v < lb || v > ub {
			t.Errorf("Rare: out of range %s in cards distribution, %f", c, v)
			t.Errorf("Range: %f, %f", lb, ub)
		}
	}
	for _, c := range products.OccultaNovelliaKindaRare() {
		target := float64(cardIters) * 0.24 / float64(len(products.OccultaNovelliaKindaRare()))
		v := float64(randCards[c])
		lb := target * (1 - toleranceFactor)
		ub := target * (1 + toleranceFactor)
		if  v < lb || v > ub {
			t.Errorf("KindaRare: out of range %s in cards distribution, %f", c, v)
			t.Errorf("Range: %f, %f", lb, ub)
		}
	}
	for _, c := range products.OccultaNovelliaNotThatRare() {
		target := float64(cardIters) * 0.75 / float64(len(products.OccultaNovelliaNotThatRare()))
		v := float64(randCards[c])
		lb := target * (1 - toleranceFactor)
		ub := target * (1 + toleranceFactor)
		if  v < lb || v > ub {
			t.Errorf("NotThatRare: out of range %s in cards distribution, %f", c, v)
			t.Errorf("Range: %f, %f", lb, ub)
		}
	}

	t.Errorf("Cards distribution: %+v", randCards)
}
