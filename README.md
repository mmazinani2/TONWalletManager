# TONWalletManager
This Go script provides a comprehensive solution for managing TON (The Open Network) wallets. It includes functionalities for creating new wallets, checking balances, deploying contracts, and automating the sending of transactions from a receiver file. The script leverages the tonutils-go library to interact with the TON blockchain.

## Features
Create New Wallet: Generate a new TON wallet with a unique seed phrase and address. The wallet address is saved as a QR code for easy sharing.
Check Wallet Balance: Retrieve and display the balance of a configured wallet.
Deploy Contract and Activate Wallet: Deploy and activate the wallet with a zero-amount transaction.
Send Transactions from Receiver File: Automatically read receiver details from a file and send transactions to multiple addresses. Supports adding a comment to each transaction.
Configuration Management: Easily read and write configuration settings from/to a text file.

## Installation
1. Install the required dependencies:

> go get ./...

## Usage
> go run main.go
 
## Configuration
The config.txt file stores the wallet seed phrase, comment for transactions, and folder path for transaction files. Example config.txt:


seed=your-seed-phrase
comment=Transaction comment
folder_path=./transactions


## License
This project is licensed under the MIT License.

## Contributing
Contributions are welcome! Please open an issue or submit a pull request.

## Acknowledgements
tonutils-go: A Go library for interacting with the TON blockchain.

## This is my TON address: 

``` 
UQC6314CqvFzKo1hCWI3glBIAx0J1gWVoYVPzp_KD6BXJqqO

If you appreciate my work and would like to support me, I would be very grateful for your generosity. Thank you!