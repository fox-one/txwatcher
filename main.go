package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fox-one/mixin-sdk-go"
)

var (
	configPath = flag.String("config","","keystore path")
)

func loadKeystore() (*mixin.Keystore,error) {
	f,err := os.Open(*configPath)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	var keystore mixin.Keystore
	if err := json.NewDecoder(f).Decode(&keystore);err != nil {
		return nil, err
	}

	return &keystore,err
}

func main() {
	flag.Parse()

	store,err := loadKeystore()
	if err != nil {
		log.Fatalln("loadKeystore",err)
	}

	client,err := mixin.NewFromKeystore(store)
	if err != nil {
		log.Fatalln("init mixin client",err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	offset := time.Unix(0,0)
	const limit = 500

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
			outputs,err := client.ReadMultisigs(ctx,offset,limit)
			if err != nil {
				log.Println("ReadMultisigs",err)
				break
			}

			for _,output := range outputs {
				offset = output.CreatedAt

				if err := handleOutput(ctx,client,output);err != nil {
					log.Println("handleOutput",output.UTXOID,output.State,err)
				}
			}

			if len(outputs) < limit {
				offset = time.Unix(0,0)
			}
		}
	}
}

func handleOutput(ctx context.Context,client *mixin.Client,output *mixin.MultisigUTXO) error {
	if output.State != mixin.UTXOStateSigned {
		return nil
	}

	tx,err := mixin.TransactionFromRaw(output.SignedTx)
	if err != nil {
		return fmt.Errorf("TransactionFromRaw: %w",err)
	}

	if tx.AggregatedSignature != nil {
		if _, err := client.SendRawTransaction(ctx, output.SignedTx); err != nil {
			return fmt.Errorf("SendRawTransaction: %w", err)
		}
	}

	return nil
}