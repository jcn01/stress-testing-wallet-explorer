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
	baseUrl          = "https://api-2.testnet2.xpxsirius.io"
	LOCK_FUND        = 10
	LOCK_FUND_TX_FEE = 26.7
)

var ctx context.Context
var client *sdk.Client
var account *sdk.Account
var cosigner *sdk.Account
var cosignerPrivateKey *string
var cosignatories []*sdk.Account
var accounts []*sdk.Account
var trxs []sdk.Transaction

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

	publicKeys := readFile()
	multisigAccounts := getAccounts(publicKeys)

	for i := range multisigAccounts {
		trx := addCosigner(multisigAccounts[i])
		trxs = append(trxs, trx)
		if i != 0 {
			cosignatories = append(cosignatories, cosigner)
			cosignatories = append(cosignatories, multisigAccounts[i])
		}
	}

	aggBondedTx := createAggregateBondedTx(trxs)

	//first account in .txt file will pay for lock funds and tx fee
	firstMultisigAccInfo := getAccountInfo(multisigAccounts[0])
	isSufficient := isXpxBalanceSufficient(aggBondedTx, firstMultisigAccInfo)

	if !isSufficient {
		fmt.Printf("Not enough XPX balance!")
	} else if isSufficient {
		fmt.Println("Enough XPX balance, adding cosigner to accounts...")
		signAggBondedTxWithCosignatures(aggBondedTx, multisigAccounts[0], cosignatories)
	}

}

func addCosigner(multisigAcc *sdk.Account) *sdk.ModifyMultisigAccountTransaction {

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

	return convertIntoMultisigTx
}

func createAggregateBondedTx(trxs []sdk.Transaction) *sdk.AggregateTransaction {

	aggBondedTx, err := client.NewBondedAggregateTransaction(
		sdk.NewDeadline(time.Hour),
		trxs,
	)
	if err != nil {
		panic(err)
	}

	return aggBondedTx
}

func signAggBondedTxWithCosignatures(aggTx *sdk.AggregateTransaction, account *sdk.Account, cosignatories []*sdk.Account) {

	signedTx, err := account.SignWithCosignatures(aggTx, cosignatories)
	if err != nil {
		fmt.Printf("Sign returned error: %s", err)
	}

	createLockFundsTx(signedTx, account)

	signedTxHash, err := client.Transaction.Announce(ctx, signedTx)
	if err != nil {
		fmt.Printf("Transaction.Announce returned error: %s", err)
	}

	fmt.Printf("Tx Hash\t: %s\n", signedTxHash)
}

func createLockFundsTx(signedTx *sdk.SignedTransaction, account *sdk.Account) {

	lockFundsTx, err := client.NewLockFundsTransaction(
		sdk.NewDeadline(time.Hour*1),
		sdk.XpxRelative(10),
		sdk.Duration(100),
		signedTx,
	)
	if err != nil {
		fmt.Printf("NewLockFundsTransaction returned error: %s", err)
	}

	signedlockFundsTx, err := account.Sign(lockFundsTx)
	if err != nil {
		fmt.Printf("Sign returned error: %s", err)
	}

	_, err = client.Transaction.Announce(ctx, signedlockFundsTx)
	if err != nil {
		fmt.Printf("Transaction.Announce returned error: %s", err)
	}

	time.Sleep(30 * time.Second)
}

func getAccount(privateKey string) *sdk.Account {

	account, err := client.NewAccountFromPrivateKey(privateKey)
	if err != nil {
		fmt.Printf("NewAccountFromPublicKey returned error: %s", err)
	}

	return account
}

func getAccounts(pKeys []string) []*sdk.Account {

	for _, pKey := range pKeys {
		account = getAccount(pKey)
		accounts = append(accounts, account)
	}

	return accounts
}

func getAccountInfo(account *sdk.Account) *sdk.AccountInfo {

	accInfo, err := client.Account.GetAccountInfo(ctx, account.Address)
	if err != nil {
		fmt.Printf("GetAccountInfo returned error: %s", err)
	}

	return accInfo
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

func getTotalFee(aggTx *sdk.AggregateTransaction) float64 {
	totalFee := LOCK_FUND + LOCK_FUND_TX_FEE + getTotalAggTxFee(aggTx)

	return totalFee
}

func isXpxBalanceSufficient(aggTx *sdk.AggregateTransaction, accInfo *sdk.AccountInfo) bool {
	fmt.Printf("Current balance\t: %v\n", getXpxBalanceByAccount(accInfo))
	fmt.Printf("Total AggTxFee\t: %v\n", getTotalAggTxFee(aggTx))
	fmt.Printf("Total Fee\t: %v\n\n", getTotalFee(aggTx))

	return getTotalFee(aggTx) <= getXpxBalanceByAccount(accInfo)
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
