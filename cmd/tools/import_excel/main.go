package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"era-inventory-api/pkg/importer"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: import_excel --file=path.xlsx --org-id=... --site-id=... --mapping=configs/mapping/mbip_equipment.yaml")
		os.Exit(1)
	}

	var filePath, orgIDStr, siteIDStr, mappingPath string
	dryRun := false

	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--file=") {
			filePath = strings.TrimPrefix(arg, "--file=")
		} else if strings.HasPrefix(arg, "--org-id=") {
			orgIDStr = strings.TrimPrefix(arg, "--org-id=")
		} else if strings.HasPrefix(arg, "--site-id=") {
			siteIDStr = strings.TrimPrefix(arg, "--site-id=")
		} else if strings.HasPrefix(arg, "--mapping=") {
			mappingPath = strings.TrimPrefix(arg, "--mapping=")
		} else if arg == "--dry-run" {
			dryRun = true
		}
	}

	if filePath == "" || orgIDStr == "" || siteIDStr == "" {
		fmt.Println("Error: file, org-id, and site-id are required")
		fmt.Println("Usage: import_excel --file=path.xlsx --org-id=... --site-id=... [--mapping=...] [--dry-run]")
		os.Exit(1)
	}

	orgID, err := strconv.ParseInt(orgIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid org-id: %v", err)
	}

	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid site-id: %v", err)
	}

	// Connect to database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/era?sslmode=disable"
	}

	db, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Open Excel file
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to open Excel file: %v", err)
	}
	defer file.Close()

	fmt.Printf("Importing from %s to org_id=%d, site_id=%d (dry_run=%v)\n", filePath, orgID, siteID, dryRun)
	fmt.Println("=" + strings.Repeat("=", 60))

	// Import using the library
	summary, err := importer.ImportExcel(context.Background(), db, file, importer.ImportOptions{
		OrgID:       orgID,
		SiteID:      siteID,
		MappingPath: mappingPath,
		DryRun:      dryRun,
		MaxErrors:   50,
	})

	if err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	// Print summary
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("IMPORT SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("Total inserted: %d\n", summary.Inserted)
	fmt.Printf("Total updated: %d\n", summary.Updated)
	fmt.Printf("Total skipped: %d\n", summary.Skipped)
	fmt.Printf("Total errors: %d\n", summary.Errors)
	fmt.Printf("Dry run: %v\n", summary.DryRun)

	if len(summary.Sheets) > 0 {
		fmt.Println("\nSheet Details:")
		for _, sheet := range summary.Sheets {
			fmt.Printf("  %s: inserted=%d, updated=%d, skipped=%d, errors=%d\n",
				sheet.Name, sheet.Inserted, sheet.Updated, sheet.Skipped, sheet.Errors)

			if len(sheet.Samples) > 0 {
				fmt.Printf("    Error samples:\n")
				for _, sample := range sheet.Samples {
					fmt.Printf("      Row %d: %s\n", sample.Row, sample.Message)
				}
			}
		}
	}
}
