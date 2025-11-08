package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/masato25/aika-dba/internal/app"
	"github.com/masato25/aika-dba/pkg/preparer"
	"github.com/masato25/aika-dba/pkg/query"
)

func main() {
	var command = flag.String("command", "", "Command to run: prepare, query")
	var question = flag.String("question", "", "Question for query command")
	flag.Parse()

	// 創建應用程序實例
	app, err := app.NewApp()
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	defer app.Close()

	switch *command {
	case "prepare":
		runPrepare(app)
	case "query":
		if *question == "" {
			log.Fatal("Question is required for query command")
		}
		runQuery(app, *question)
	default:
		fmt.Println("Usage:")
		fmt.Println("  -command prepare                    # Prepare knowledge base")
		fmt.Println("  -command query -question 'your question'  # Query knowledge base")
		os.Exit(1)
	}
}

func runPrepare(app *app.App) {
	app.Logger.Println("Starting knowledge base preparation...")

	preparer := preparer.NewKnowledgePreparer(app.DB, app.KnowledgeManager, app.Logger)
	if err := preparer.PrepareKnowledge(); err != nil {
		log.Fatalf("Knowledge preparation failed: %v", err)
	}

	app.Logger.Println("Knowledge base preparation completed successfully")
}

func runQuery(app *app.App, question string) {
	app.Logger.Printf("Processing query: %s", question)

	queryInterface := query.NewQueryInterface(app.KnowledgeManager, app.LLMClient, app.DB, app.Logger)
	answer, err := queryInterface.Query(question)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}

	fmt.Println("\nAnswer:")
	fmt.Println(answer)
}
