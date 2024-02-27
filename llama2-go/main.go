package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	nn "github.com/nikolaydubina/llama2.go/exp/nnfast"
	"github.com/nikolaydubina/llama2.go/llama2"
)

func main() {

	var (
		checkpointFilePath string
		tokenizerFilePath  string
		temperature        float64
		steps              int
		topp               float64
	)

	flag.StringVar(&checkpointFilePath, "checkpoint", "stories110M.bin", "checkpoint binary file with weights")
	flag.StringVar(&tokenizerFilePath, "tokenizer", "tokenizer.bin", "tokenizer binary file with vocabulary (get it from repo)")
	flag.Float64Var(&temperature, "temperature", 0.9, "temperature (optional; 0 = deterministic argmax sampling; 1 = baseline)")
	flag.IntVar(&steps, "steps", 256, "max number of steps to run for, 0: use seq_len")
	flag.Float64Var(&topp, "topp", 0.9, "top-p in nucleus sampling (1.0 = off; 0.9 works well, but slower)")
	flag.Parse()

	checkpointFile, err := os.OpenFile(checkpointFilePath, os.O_RDONLY, 0)
	if err != nil {
		log.Fatal("打开文件失败:%#v", err)
		// log.Fatal(err)
	}
	defer checkpointFile.Close()

	config, err := llama2.NewConfigFromCheckpoint(checkpointFile)
	if err != nil {
		log.Fatalf("cannot read config: %s", err)
	}
	log.Printf("config: %#v\n", config)

	isSharedWeights := config.VocabSize > 0
	if config.VocabSize < 0 {
		config.VocabSize = -config.VocabSize
		log.Printf("shared weights: %v\n", isSharedWeights)
	}

	tokenizerFile, err := os.OpenFile(tokenizerFilePath, os.O_RDONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tokenizer: %s\n", tokenizerFilePath)
	defer tokenizerFile.Close()

	vocab := llama2.NewVocabFromFile(config.VocabSize, tokenizerFile)

	 log.Printf("vocab size: %v\n", config.VocabSize)
	w := llama2.NewTransformerWeightsFromCheckpoint(config, checkpointFile, isSharedWeights)

	log.Printf("weights: %v\n", w)
	if steps <= 0 || steps > config.SeqLen {
		log.Printf("using seq_len: %v\n", config.SeqLen)
		steps = config.SeqLen
	}

	log.Printf("temperature: %v\n", temperature)
	runState := llama2.NewRunState(config)

	

	for {
		fmt.Print("Enter a prompt (type 'exit' to quit): ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		userInput := scanner.Text()

		if userInput == "exit" {
			fmt.Println("Exiting...")
			break
		}

		promptTokens := vocab.Encode(userInput)

		timeStart := time.Now()
		var token int = 1
		var pos = 0
		for pos < steps {
			llama2.Transformer(token, pos, config, runState, w)

			var next int
			if pos < len(promptTokens) {
				next = promptTokens[pos]
			} else {
				if temperature == 0 {
					next = nn.ArgMax(runState.Logits)
				} else {
					for q := 0; q < config.VocabSize; q++ {
						runState.Logits[q] /= float32(temperature)
					}
					nn.SoftMax(runState.Logits)
					if topp <= 0 || topp >= 1 {
						next = nn.Sample(runState.Logits)
					} else {
						next = nn.SampleTopP(runState.Logits, float32(topp))
					}
				}
			}
			pos++

			if next == 1 {
				break
			}

			var tokenStr string
			if token == 1 && vocab.Words[next][0] == ' ' {
				tokenStr = vocab.Words[next][1:]
			} else {
				tokenStr = vocab.Words[next]
			}
			fmt.Print(tokenStr)

			token = next
		}
		fmt.Println("\nAchieved tok/s:", float64(pos-1)/time.Since(timeStart).Seconds())
	}
}



