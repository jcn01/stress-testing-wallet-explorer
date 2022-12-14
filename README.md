# Golang scripts for stress testing

### Create account with many asset
```
go run main.go -key account_private_key -num number_of_assets_to_be_created
```


### Create multi-level multisig account
Provide list of account private keys in `.txt` file
```
go run main.go
```


### Add cosigner to accounts
Provide list of account private keys in `.txt` file
```
go run main.go -key cosigner_account_private_key
```

<br/>

*Note: First account in `.txt` file will pay for lock funds and transaction fees.*
