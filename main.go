package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

	"github.com/proximax-storage/go-xpx-chain-sdk/sdk"
)

const (
	baseUrl = "https://api-2.testnet2.xpxsirius.io"
)

var client *sdk.Client
var account *sdk.Account
var privateKey *string
var number *int

func init() {
	privateKey = flag.String("key", "", "Account private key")
	number = flag.Int("num", 0, "Number of assets to be created")
	flag.Parse()

	conf, err := sdk.NewConfig(context.Background(), []string{baseUrl})
	if err != nil {
		fmt.Printf("NewConfig returned error: %s", err)
		return
	}

	client = sdk.NewClient(nil, conf)

	account, err = client.NewAccountFromPrivateKey(*privateKey)
	if err != nil {
		fmt.Printf("NewAccountFromPrivateKey returned error: %s", err)
		return
	}

}

func main() {
	for i := 0; i < (*number); i++ {
		createAsset()
		// wait for the transaction to be confirmed
		time.Sleep(30 * time.Second)

	}
}

func createAsset() {
	nonce := rand.New(rand.NewSource(time.Now().UTC().UnixNano())).Uint32()

	//Mosaic defination transaction
	mosaicDefinitionTx, err := client.NewMosaicDefinitionTransaction(
		sdk.NewDeadline(time.Hour),
		nonce,
		account.PublicAccount.PublicKey,
		sdk.NewMosaicProperties(true, true, 0, sdk.Duration(0)),
	)

	if err != nil {
		fmt.Printf("NewMosaicDefinitionTransaction returned error: %s", err)
		return
	}

	mosaic, err := sdk.NewMosaicIdFromNonceAndOwner(nonce, account.PublicAccount.PublicKey)
	if err != nil {
		panic(err)
	}

	//Mosaic supply change transaction
	mosaicSupplyChangeTx, err := client.NewMosaicSupplyChangeTransaction(
		sdk.NewDeadline(time.Hour),
		mosaic,
		sdk.Increase,
		sdk.Amount(1),
	)
	if err != nil {
		panic(err)
	}

	mosaicDefinitionTx.ToAggregate(account.PublicAccount)
	mosaicSupplyChangeTx.ToAggregate(account.PublicAccount)

	// Create an aggregate complete transaction
	aggregateTx, err := client.NewCompleteAggregateTransaction(
		sdk.NewDeadline(time.Hour),
		[]sdk.Transaction{
			mosaicDefinitionTx,
			mosaicSupplyChangeTx},
	)

	if err != nil {
		fmt.Printf("NewCompleteAggregateTransaction returned error: %s", err)
		return
	}

	// Sign transaction
	signedTx, err := account.Sign(aggregateTx)
	if err != nil {
		fmt.Printf("Sign returned error: %s", err)
		return
	}

	// Announce transaction
	_, err = client.Transaction.Announce(context.Background(), signedTx)
	if err != nil {
		fmt.Printf("Transaction.Announce returned error: %s", err)
		return
	}

	fmt.Println(signedTx.Hash.String())

}
