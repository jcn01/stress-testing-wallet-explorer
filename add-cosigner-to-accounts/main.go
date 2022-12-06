package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/proximax-storage/go-xpx-chain-sdk/sdk"
)

const (
	baseUrl = "https://api-2.testnet2.xpxsirius.io"
	FEE     = 67.3
)

var ctx context.Context
var client *sdk.Client
var cosigner *sdk.Account
var cosignerPrivateKey *string

func init() {
	cosignerPrivateKey = flag.String("key", "", "Cosigner account private key")
	flag.Parse()

	ctx = context.Background()

	conf, err := sdk.NewConfig(ctx, []string{baseUrl})
	if err != nil {
		fmt.Printf("NewConfig returned error: %s", err)
	}

	client = sdk.NewClient(nil, conf)

	cosigner, err = client.NewAccountFromPrivateKey(*cosignerPrivateKey)
	if err != nil {
		fmt.Printf("NewAccountFromPrivateKey returned error: %s", err)
	}
}

func main() {

	privateKeys := readFile()

	for i := range privateKeys {
		multisigAcc, accInfo := getAccount(privateKeys[i])
		isSufficient := isXpxBalanceSufficient(accInfo)

		if !isSufficient {
			fmt.Printf("Not enough XPX balance! Account: %s\n", multisigAcc.Address.String())
			continue
		} else if isSufficient {
			fmt.Println("Enough XPX balance, adding cosigner to account...")
			createMultisigAccount(multisigAcc)
		}
	}

}

func createMultisigAccount(multisigAcc *sdk.Account) {
	//New modify multisig account transaction
	convertIntoMultisigTx, err := client.NewModifyMultisigAccountTransaction(
		sdk.NewDeadline(time.Hour),
		1,
		1,
		[]*sdk.MultisigCosignatoryModification{{
			Type:          sdk.Add,
			PublicAccount: cosigner.PublicAccount,
		}},
	)
	if err != nil {
		panic(err)
	}

	convertIntoMultisigTx.ToAggregate(multisigAcc.PublicAccount)

	aggBondedTx, err := client.NewBondedAggregateTransaction(
		sdk.NewDeadline(time.Hour),
		[]sdk.Transaction{convertIntoMultisigTx},
	)
	if err != nil {
		panic(err)
	}

	signedAggBondedTx, err := multisigAcc.Sign(aggBondedTx)
	if err != nil {
		panic(err)
	}

	{ //Lock funds transaction
		lockFundsTx, err := client.NewLockFundsTransaction(sdk.NewDeadline(time.Hour*1), sdk.XpxRelative(10), sdk.Duration(100), signedAggBondedTx)
		if err != nil {
			fmt.Printf("NewLockFundsTransaction returned error: %s", err)
		}

		// Future multisig account will pay for the lock funds
		signedlockFundsTx, err := multisigAcc.Sign(lockFundsTx)
		if err != nil {
			fmt.Printf("Sign returned error: %s", err)
		}

		_, err = client.Transaction.Announce(context.Background(), signedlockFundsTx)
		if err != nil {
			fmt.Printf("Transaction.Announce returned error: %s", err)
		}

		time.Sleep(30 * time.Second)
	}

	_, err = client.Transaction.AnnounceAggregateBonded(context.Background(), signedAggBondedTx)
	if err != nil {
		fmt.Printf("Transaction.AnnounceAggregateBonded returned error: %s", err)
		return
	}

	time.Sleep(30 * time.Second)

	//New cosignature transaction
	cosignatureTx := sdk.NewCosignatureTransactionFromHash(signedAggBondedTx.Hash)
	signedCosignatureTx, err := cosigner.SignCosignatureTransaction(cosignatureTx)
	if err != nil {
		fmt.Printf("SignCosignatureTransaction returned error: %s", err)
		return
	}

	_, err = client.Transaction.AnnounceAggregateBondedCosignature(context.Background(), signedCosignatureTx)
	if err != nil {
		fmt.Printf("AnnounceAggregateBoundedCosignature returned error: %s", err)
		return
	}

	time.Sleep(30 * time.Second)
}

func getAccount(privateKey string) (*sdk.Account, *sdk.AccountInfo) {
	acc, err := client.NewAccountFromPrivateKey(privateKey)
	if err != nil {
		fmt.Printf("NewAccountFromPrivateKey returned error: %s", err)
	}

	accInfo, err := client.Account.GetAccountInfo(ctx, acc.Address)
	if err != nil {
		fmt.Printf("GetAccountInfo returned error: %s", err)
	}

	return acc, accInfo
}

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

func isXpxBalanceSufficient(accInfo *sdk.AccountInfo) bool {
	xpxBalance := getXpxBalanceByAccount(accInfo)

	fmt.Printf("Current balance\t: %v\n", xpxBalance)
	fmt.Printf("Total fee\t: %v\n", FEE)

	return FEE <= xpxBalance
}

// Read multisig acc private keys from .txt file
func readFile() []string {
	file, err := os.Open("private-keys-sample.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	pKeys := make([]string, 0)

	for scanner.Scan() {
		pKeys = append(pKeys, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return pKeys
}
