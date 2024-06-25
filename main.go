package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"github.com/xssnick/tonutils-go/tvm/cell"
)

func readConfig(filePath string) (map[string]string, error) {
	config := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[parts[0]] = parts[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return config, nil
}

func writeConfig(filePath string, config map[string]string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	for key, value := range config {
		_, err := file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			return err
		}
	}

	return nil
}

func readReceivers(filePath string) (map[string]string, error) {
	receivers := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 {
			receivers[parts[0]] = parts[1]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return receivers, nil
}

func createNewWallet() (*wallet.Wallet, string, string, string, string, error) {
	client := liteclient.NewConnectionPool()
	configUrl := "https://ton.org/global.config.json"
	err := client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
	if err != nil {
		return nil, "", "", "", "", err
	}

	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()

	seed := wallet.NewSeed()
	if seed == nil {
		return nil, "", "", "", "", nil
	}

	w, err := wallet.FromSeed(api, seed, wallet.ConfigHighloadV3{
		MessageTTL: 60 * 5,
		MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
			createdAt = time.Now().Unix() - 30
			return uint32(createdAt % (1 << 23)), createdAt, nil
		},
	})
	if err != nil {
		return nil, "", "", "", "", err
	}

	seedStr := strings.Join(seed, " ")
	walletAddr := w.WalletAddress().String()

	// Create QR code
	qrCode, err := qrcode.New(walletAddr, qrcode.Medium)
	if err != nil {
		return nil, "", "", "", "", err
	}

	qrCodeFile := "wallet_qr.png"
	err = qrCode.WriteFile(256, qrCodeFile)
	if err != nil {
		return nil, "", "", "", "", err
	}

	fmt.Println("New wallet created!")
	fmt.Println("Seed:", seedStr)
	fmt.Println("Wallet Address:", walletAddr)
	fmt.Println("QR Code saved to:", qrCodeFile)

	return w, seedStr, "", "", walletAddr, nil
}

func autoSend() {
	for {
		config, err := readConfig("config.txt")
		if err != nil {
			log.Println("Error reading config:", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		seed := strings.Split(config["seed"], " ")
		commentText := config["comment"]
		folderPath := config["folder_path"]

		client := liteclient.NewConnectionPool()
		configUrl := "https://ton.org/global.config.json"
		err = client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
		if err != nil {
			log.Println("Error adding connections:", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()

		w, err := wallet.FromSeed(api, seed, wallet.ConfigHighloadV3{
			MessageTTL: 60 * 5,
			MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
				createdAt = time.Now().Unix() - 30
				return uint32(createdAt % (1 << 23)), createdAt, nil
			},
		})
		if err != nil {
			log.Println("FromSeed err:", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		block, err := api.CurrentMasterchainInfo(context.Background())
		if err != nil {
			log.Println("CurrentMasterchainInfo err:", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		balance, err := w.GetBalance(context.Background(), block)
		if err != nil {
			log.Println("GetBalance err:", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		files, err := filepath.Glob(filepath.Join(folderPath, "*.txt"))
		if err != nil {
			log.Println("Error reading folder:", err)
			time.Sleep(1 * time.Minute)
			continue
		}

		if len(files) == 0 {
			time.Sleep(1 * time.Minute)
			log.Println("File not exist")
			continue
		}

		for _, file := range files {
			receivers, err := readReceivers(file)
			if err != nil {
				log.Println("Error reading receivers:", err)
				continue
			}

			if balance.Nano().Uint64() < 300000 {
				log.Println("Not enough balance:", balance.String(), balance.Nano().String())
				break
			}

			comment, err := wallet.CreateCommentCell(commentText)
			if err != nil {
				log.Println("CreateComment err:", err)
				continue
			}

			var messages []*wallet.Message
			for addrStr, amtStr := range receivers {
				addrParsed, err := address.ParseAddr(addrStr)
				if err != nil {
					log.Printf("Error parsing address %s: %v", addrStr, err)
					continue
				}

				amount, err := tlb.FromTON(amtStr)
				if err != nil {
					log.Printf("Error parsing amount %s: %v", amtStr, err)
					continue
				}

				messages = append(messages, &wallet.Message{
					Mode: 1 + 2,
					InternalMessage: &tlb.InternalMessage{
						IHRDisabled: true,
						Bounce:      addrParsed.IsBounceable(),
						DstAddr:     addrParsed,
						Amount:      amount,
						Body:        comment,
					},
				})
			}

			log.Println("Sending transaction and waiting for confirmation...")

			txHash, err := w.SendManyWaitTxHash(context.Background(), messages)
			if err != nil {
				log.Println("Transfer err:", err)
				continue
			}

			newFileName := fmt.Sprintf("%s_%s.log", filepath.Base(file), base64.StdEncoding.EncodeToString(txHash)[:8])
			newFilePath := filepath.Join(filepath.Dir(file), newFileName)
			err = os.Rename(file, newFilePath)
			if err != nil {
				log.Println("Error renaming file:", err)
				continue
			}

			log.Println("Transaction sent, hash:", base64.StdEncoding.EncodeToString(txHash))
			log.Println("File renamed to:", newFilePath)
		}

		time.Sleep(1 * time.Minute)
	}
	time.Sleep(1 * time.Minute)
	go autoSend()
}

func main() {
	// go autoSend()

	for {
		fmt.Println("1. Create new wallet")
		fmt.Println("2. Show wallet balance")
		fmt.Println("3. Deploy contract and activate wallet")
		fmt.Println("4. Send transactions from receiver file")
		fmt.Println("5. Exit")

		var choice int
		fmt.Scan(&choice)

		switch choice {
		case 1:
			_, seed, _, _, walletAddr, err := createNewWallet()

			if err != nil {
				log.Fatal("Error creating wallet:", err)
			}

			config, err := readConfig("config.txt")
			if err != nil {
				config = make(map[string]string)
			}
			config["seed"] = seed
			config["wallet_address"] = walletAddr

			err = writeConfig("config.txt", config)
			if err != nil {
				log.Fatal("Error writing config:", err)
			}
		case 2:
			config, err := readConfig("config.txt")
			if err != nil {
				log.Fatal("Error reading config:", err)
			}

			seed := strings.Split(config["seed"], " ")

			client := liteclient.NewConnectionPool()
			configUrl := "https://ton.org/global.config.json"
			err = client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
			if err != nil {
				log.Fatal("Error adding connections:", err)
			}

			api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()

			w, err := wallet.FromSeed(api, seed, wallet.ConfigHighloadV3{
				MessageTTL: 60 * 5,
				MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
					createdAt = time.Now().Unix() - 30
					return uint32(createdAt % (1 << 23)), createdAt, nil
				},
			})
			if err != nil {
				log.Fatal("FromSeed err:", err)
			}

			block, err := api.CurrentMasterchainInfo(context.Background())
			if err != nil {
				log.Fatal("CurrentMasterchainInfo err:", err)
			}

			balance, err := w.GetBalance(context.Background(), block)
			if err != nil {
				log.Fatal("GetBalance err:", err)
			}

			fmt.Println("Balance:", balance.String())
			fmt.Println("Wallet address:", w.WalletAddress().String())
		case 3:
			config, err := readConfig("config.txt")
			if err != nil {
				log.Fatal("Error reading config:", err)
			}

			seed := strings.Split(config["seed"], " ")

			client := liteclient.NewConnectionPool()
			configUrl := "https://ton.org/global.config.json"
			err = client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
			if err != nil {
				log.Fatal("Error adding connections:", err)
			}

			api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()

			w, err := wallet.FromSeed(api, seed, wallet.ConfigHighloadV3{
				MessageTTL: 60 * 5,
				MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
					createdAt = time.Now().Unix() - 30
					return uint32(createdAt % (1 << 23)), createdAt, nil
				},
			})
			if err != nil {
				log.Fatalln("FromSeed err:", err.Error())

			}

			block, err := api.CurrentMasterchainInfo(context.Background())
			if err != nil {
				log.Fatalln("CurrentMasterchainInfo err:", err.Error())
			}
			balance, err := w.GetBalance(context.Background(), block)
			if err != nil {
				log.Fatalln("GetBalance err:", err.Error())

			}

			// create empty body cell
			body := cell.BeginCell().EndCell()

			log.Println("sending transaction and waiting for confirmation...")

			tx, block, err := w.SendWaitTransaction(context.Background(), &wallet.Message{
				Mode: 1, // pay fees separately (from balance, not from amount)
				InternalMessage: &tlb.InternalMessage{
					Bounce:  true, // return amount in case of processing error
					DstAddr: w.WalletAddress(),
					Amount:  tlb.MustFromTON("0"),
					Body:    body,
				},
			})
			if err != nil {
				log.Fatalln("Send err:", err.Error())

			}

			log.Println("transaction sent, confirmed at block, hash:", base64.StdEncoding.EncodeToString(tx.Hash))

			balance, err = w.GetBalance(context.Background(), block)
			if err != nil {
				log.Fatalln("GetBalance err:", err.Error())

			}

			log.Println("balance left:", balance.String())

		case 4:
			config, err := readConfig("config.txt")
			if err != nil {
				log.Fatal("Error reading config:", err)
			}

			seed := strings.Split(config["seed"], " ")
			commentText := config["comment"]

			client := liteclient.NewConnectionPool()
			configUrl := "https://ton.org/global.config.json"
			err = client.AddConnectionsFromConfigUrl(context.Background(), configUrl)
			if err != nil {
				log.Fatal("Error adding connections:", err)
			}

			api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()

			w, err := wallet.FromSeed(api, seed, wallet.ConfigHighloadV3{
				MessageTTL: 60 * 5,
				MessageBuilder: func(ctx context.Context, subWalletId uint32) (id uint32, createdAt int64, err error) {
					createdAt = time.Now().Unix() - 30
					return uint32(createdAt % (1 << 23)), createdAt, nil
				},
			})
			if err != nil {
				log.Fatal("FromSeed err:", err)
			}

			block, err := api.CurrentMasterchainInfo(context.Background())
			if err != nil {
				log.Fatal("CurrentMasterchainInfo err:", err)
			}

			balance, err := w.GetBalance(context.Background(), block)
			if err != nil {
				log.Fatal("GetBalance err:", err)
			}

			receivers, err := readReceivers("receivers.txt")
			if err != nil {
				log.Fatal("Error reading receivers:", err)
			}

			if balance.Nano().Uint64() >= 300000 {
				comment, err := wallet.CreateCommentCell(commentText)
				if err != nil {
					log.Fatal("CreateComment err:", err)
				}

				var messages []*wallet.Message
				for addrStr, amtStr := range receivers {
					addrParsed, err := address.ParseAddr(addrStr)
					if err != nil {
						log.Fatalf("Error parsing address %s: %v", addrStr, err)
						continue
					}

					amount, err := tlb.FromTON(amtStr)
					if err != nil {
						log.Fatalf("Error parsing amount %s: %v", amtStr, err)
						continue
					}

					messages = append(messages, &wallet.Message{
						Mode: 1 + 2,
						InternalMessage: &tlb.InternalMessage{
							IHRDisabled: true,
							Bounce:      addrParsed.IsBounceable(),
							DstAddr:     addrParsed,
							Amount:      amount,
							Body:        comment,
						},
					})
				}

				fmt.Println("Sending transaction and waiting for confirmation...")

				txHash, err := w.SendManyWaitTxHash(context.Background(), messages)
				if err != nil {
					log.Fatal("Transfer err:", err)
				}

				fmt.Println("Transaction sent, hash:", base64.StdEncoding.EncodeToString(txHash))
			} else {
				fmt.Println("Not enough balance:", balance.String(), balance.Nano().String())
			}
		case 5:
			return
		default:
			fmt.Println("Invalid choice")
		}
	}
}
