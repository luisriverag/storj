// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"storj.io/storj/shared/dbutil/dbschema"
)

func findTestData(glob string) (version int, data string, err error) {
	// find all testdata sql files
	matches, err := filepath.Glob(glob)
	if err != nil {
		panic(err)
	}

	sort.Slice(matches, func(i, k int) bool {
		return parseTestdataVersion(matches[i]) < parseTestdataVersion(matches[k])
	})

	lastScriptFile := matches[len(matches)-1]
	version = parseTestdataVersion(lastScriptFile)
	if version < 0 {
		return 0, "", fmt.Errorf("invalid version " + lastScriptFile)
	}

	scriptData, err := os.ReadFile(lastScriptFile)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read file %q: %w", lastScriptFile, err)
	}

	sections := dbschema.NewSections(string(scriptData))
	data = sections.LookupSection(dbschema.Main)

	return version, data, nil
}

func main() {
	pgVersion, pgData, err := findTestData("testdata/postgres.*")
	if err != nil {
		panic(err)
	}
	spannerVersion, spannerData, err := findTestData("testdata/spanner.*")
	if err != nil {
		panic(err)
	}

	var buffer bytes.Buffer
	_, _ = fmt.Fprintf(&buffer, testMigrationFormat, spannerVersion, spannerData, pgVersion, pgData)

	formatted, err := format.Source(buffer.Bytes())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "source:\n%s\nerr: %s\n", buffer.Bytes(), err)
		panic(err)
	}

	err = os.WriteFile("migratez.go", formatted, 0755)
	if err != nil {
		panic(err)
	}
}

var testFilePrefix = regexp.MustCompile(`testdata/(spanner|postgres)\.v(\d+)\.sql$`)

func parseTestdataVersion(path string) int {
	path = filepath.ToSlash(strings.ToLower(path))
	matches := testFilePrefix.FindStringSubmatch(path)
	if matches != nil {
		v, err := strconv.Atoi(matches[2])
		if err == nil {
			return v
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "invalid testdata path %q\n", path)
	return -1
}

var testMigrationFormat = `// AUTOGENERATED BY migrategen.go
// DO NOT EDIT.

package satellitedb

import "storj.io/storj/private/migrate"

// testMigration returns migration that can be used for testing.
func (db *satelliteDB) testMigration() *migrate.Migration {
	if db.Name() == "spanner" {
		return db.testMigrationSpanner()
	}
	return db.testMigrationPostgres()
}

// testMigrationSpanner returns migration that can be used for testing on Spanner.
func (db *satelliteDB) testMigrationSpanner() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Testing setup",
				Version:     %d,
				Action:      migrate.SQL{` + "`%s`" + `},
			},
		},
	}
}

// testMigrationPostgres returns migration that can be used for testing on Postgres.
func (db *satelliteDB) testMigrationPostgres() *migrate.Migration {
	return &migrate.Migration{
		Table: "versions",
		Steps: []*migrate.Step{
			{
				DB:          &db.migrationDB,
				Description: "Testing setup",
				Version:     %d,
				Action:      migrate.SQL{` + "`%s`" + `},
			},
		},
	}
}
`
