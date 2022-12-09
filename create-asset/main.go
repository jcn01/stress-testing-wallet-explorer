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
	baseUrl          = "https://api-2.testnet2.xpxsirius.io"
	ASSET_RENTAL_FEE = 1000
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
	for i := 0; i < (*number); i++ {
		mosaicDefinitionTx, mosaicSupplyChangeTx := createAsset()
		innerTxs = append(innerTxs, mosaicDefinitionTx, mosaicSupplyChangeTx)
	}

	aggCompleteTx := createAggCompleteTx(innerTxs)
	isSufficient := isXpxBalanceSufficient(aggCompleteTx)

	if !isSufficient {
		fmt.Println("Not enough XPX balance!")
	} else if isSufficient && *number > 0 {
		fmt.Printf("Enough XPX balance, generating list of assets...\n\n")
		signAggCompleteTx(aggCompleteTx)
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

func createAggCompleteTx(innerTxs []sdk.Transaction) *sdk.AggregateTransaction {
	aggCompleteTx, err := client.NewCompleteAggregateTransaction(
		sdk.NewDeadline(time.Hour),
		innerTxs,
	)
	if err != nil {
		fmt.Printf("NewCompleteAggregateTransaction returned error: %s", err)
	}

	return aggCompleteTx
}

func signAggCompleteTx(aggTx *sdk.AggregateTransaction) {
	// Sign transaction
	signedTx, err := account.Sign(aggTx)
	if err != nil {
		fmt.Printf("Sign returned error: %s", err)
	}

	// Announce transaction
	_, err = client.Transaction.Announce(ctx, signedTx)
	if err != nil {
		fmt.Printf("Transaction.Announce returned error: %s", err)
	}

	fmt.Printf("Tx Hash\t: %s\n", signedTx.Hash.String())
}

func getXpxBalanceByAccount(accInfo *sdk.AccountInfo) (balance float64) {
	nsId, _ := sdk.NewNamespaceIdFromName("prx.xpx")
	xpx, _ := client.Resolve.GetMosaicInfoByAssetId(ctx, nsId)

	for _, mosaic := range accInfo.Mosaics {
		if eq, _ := mosaic.AssetId.Equals(xpx.MosaicId); eq {
			balance = float64(mosaic.Amount) / 1000000
		}
	}

	return balance
}

func getTotalAggTxFee(aggTx *sdk.AggregateTransaction) float64 {
	aggTxFee := float64(aggTx.GetAbstractTransaction().MaxFee) / 1000000

	return aggTxFee
}

func getTotalAssetRentalFee() float64 {
	totalAssetRentalFee := ASSET_RENTAL_FEE * (*number)

	return float64(totalAssetRentalFee)
}

func getTotalFee(aggTx *sdk.AggregateTransaction) float64 {
	totalFee := getTotalAssetRentalFee() + getTotalAggTxFee(aggTx)

	fmt.Printf("Current Balance\t: %v\n", getXpxBalanceByAccount(accInfo))
	fmt.Printf("Total RentalFee\t: %v\n", getTotalAssetRentalFee())
	fmt.Printf("Total AggTxFee\t: %v\n", getTotalAggTxFee(aggTx))
	fmt.Printf("Total Fee\t: %v\n\n", totalFee)

	return totalFee
}

// Check if private key account has sufficient balance
func isXpxBalanceSufficient(aggTx *sdk.AggregateTransaction) bool {

	return getTotalFee(aggTx) <= getXpxBalanceByAccount(accInfo)
}
