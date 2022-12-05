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
	TX_FEE  = 1036.3
)

var ctx context.Context
var client *sdk.Client
var account *sdk.Account
var accInfo *sdk.AccountInfo
var privateKey *string
var number *int
var innerTxs []sdk.Transaction

func init() {
	privateKey = flag.String("key", "", "Account private key")
	number = flag.Int("num", 0, "Number of assets to be created")
	flag.Parse()

	ctx = context.Background()

	conf, err := sdk.NewConfig(ctx, []string{baseUrl})
	if err != nil {
		fmt.Printf("NewConfig returned error: %s", err)
	}

	client = sdk.NewClient(nil, conf)

	account, err = client.NewAccountFromPrivateKey(*privateKey)
	if err != nil {
		fmt.Printf("NewAccountFromPrivateKey returned error: %s", err)
	}

	accInfo, err = client.Account.GetAccountInfo(ctx, account.Address)
	if err != nil {
		fmt.Printf("GetAccountInfo returned error: %s", err)
	}

}

func main() {
	isSufficient := isXpxBalanceSufficient()
	if !isSufficient {
		fmt.Println("Not enough XPX balance!")
	} else if isSufficient && *number > 0 {
		fmt.Println("Enough XPX balance, generating list of assets...")
		for i := 0; i < (*number); i++ {
			mosaicDefinitionTx, mosaicSupplyChangeTx := createAsset()
			innerTxs = append(innerTxs, mosaicDefinitionTx, mosaicSupplyChangeTx)
		}
		signCreateAssetTx(innerTxs)
	}
}

func createAsset() (*sdk.MosaicDefinitionTransaction, *sdk.MosaicSupplyChangeTransaction) {
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

	return mosaicDefinitionTx, mosaicSupplyChangeTx

}

func signCreateAssetTx(innerTxs []sdk.Transaction) {
	// Aggregate complete transaction
	aggTx, err := client.NewCompleteAggregateTransaction(
		sdk.NewDeadline(time.Hour),
		innerTxs,
	)

	if err != nil {
		fmt.Printf("NewCompleteAggregateTransaction returned error: %s", err)
	}

	// Sign transaction
	signedTx, err := account.Sign(aggTx)
	if err != nil {
		fmt.Printf("Sign returned error: %s", err)
	}

	// Announce transaction
	_, err = client.Transaction.Announce(context.Background(), signedTx)
	if err != nil {
		fmt.Printf("Transaction.Announce returned error: %s", err)
	}

	fmt.Printf("Tx hash: %s\n", signedTx.Hash.String())
}

// Get xpx balance of an account
func getXpxBalanceByAccount(accInfo *sdk.AccountInfo) (balance float64) {
	nsId, _ := sdk.NewNamespaceIdFromName("prx.xpx")
	xpx, _ := client.Resolve.GetMosaicInfoByAssetId(context.Background(), nsId)

	for _, mosaic := range accInfo.Mosaics {
		if eq, _ := mosaic.AssetId.Equals(xpx.MosaicId); eq {
			balance = float64(mosaic.Amount) / 1000000
		}
	}

	return balance
}

// Check if private key account has sufficient balance
func isXpxBalanceSufficient() bool {
	xpxBalance := getXpxBalanceByAccount(accInfo)
	totalTxFee := TX_FEE * float64(*number)
	maxAsset := int(xpxBalance / TX_FEE)

	fmt.Printf("Current balance\t: %v\n", xpxBalance)
	fmt.Printf("Total fee\t: %v\n", totalTxFee)
	fmt.Printf("Maximum asset\t: %v\n\n", maxAsset)

	return totalTxFee <= xpxBalance
}
