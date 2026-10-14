// Command dbdump prints the contents of a bs-blog SQLite database.
//
// It is a maintenance/diagnostic helper: it opens the given file read-only and
// dumps users, posts and tags so a bug report can be checked against real data.
//
//	go run ./cmd/dbdump ../.gotmp/dbdump/copy/blog.db
//	go run ./cmd/dbdump -v ../.gotmp/dbdump/copy/blog.db   # also print the body
//
// -v is what distinguishes "the article was never stored" from "the article is
// stored but the UI does not show it": without the stored body in the output
// those two look identical from the outside.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/glebarez/sqlite"
)

func main() {
	args := os.Args[1:]
	verbose := false
	if len(args) > 0 && (args[0] == "-v" || args[0] == "--verbose") {
		verbose = true
		args = args[1:]
	}
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: dbdump [-v] <path-to-blog.db>")
		os.Exit(2)
	}
	path := args[0]

	// mode=ro + query_only guarantee this can never mutate the database.
	dsn := "file:" + strings.ReplaceAll(path, "\\", "/") + "?mode=ro&_pragma=query_only(true)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fail("open", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fail("ping", err)
	}

	dumpUsers(db)
	dumpPosts(db, verbose)
	dumpTags(db)
}

func dumpUsers(db *sql.DB) {
	rows, err := db.Query(`SELECT id, username, nickname, created_at, last_login_at FROM users ORDER BY id`)
	if err != nil {
		fmt.Printf("users: query failed: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("=== users ===")
	n := 0
	for rows.Next() {
		var id int64
		var username, nickname, createdAt, lastLogin sql.NullString
		if err := rows.Scan(&id, &username, &nickname, &createdAt, &lastLogin); err != nil {
			fmt.Printf("users: scan failed: %v\n", err)
			return
		}
		n++
		fmt.Printf("  #%d username=%q nickname=%q created=%s lastLogin=%s\n",
			id, username.String, nickname.String, createdAt.String, lastLogin.String)
	}
	if n == 0 {
		fmt.Println("  (no rows)")
	}
}

func dumpPosts(db *sql.DB, verbose bool) {
	query := `SELECT id, slug, title, status, visibility, published_at, created_at, updated_at,
			length(content_html), length(content_json), substr(content_html, 1, 240)
		FROM posts ORDER BY id`
	if verbose {
		query = `SELECT id, slug, title, status, visibility, published_at, created_at, updated_at,
				length(content_html), length(content_json), content_html
			FROM posts ORDER BY id`
	}
	rows, err := db.Query(query)
	if err != nil {
		fmt.Printf("posts: query failed: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("=== posts ===")
	n := 0
	for rows.Next() {
		var id, htmlLen, jsonLen int64
		var slug, title, status, visibility, publishedAt, createdAt, updatedAt, html sql.NullString
		if err := rows.Scan(&id, &slug, &title, &status, &visibility, &publishedAt,
			&createdAt, &updatedAt, &htmlLen, &jsonLen, &html); err != nil {
			fmt.Printf("posts: scan failed: %v\n", err)
			return
		}
		n++
		fmt.Printf("  #%d status=%q visibility=%q slug=%q\n", id, status.String, visibility.String, slug.String)
		fmt.Printf("      title=%q\n", title.String)
		fmt.Printf("      publishedAt=%s createdAt=%s updatedAt=%s htmlLen=%d jsonLen=%d\n",
			publishedAt.String, createdAt.String, updatedAt.String, htmlLen, jsonLen)
		if verbose {
			fmt.Printf("      contentHtml=%q\n", html.String)
		}
	}
	if n == 0 {
		fmt.Println("  (no rows)")
	}
	fmt.Printf("  total=%d\n", n)
}

func dumpTags(db *sql.DB) {
	rows, err := db.Query(`SELECT id, name, slug FROM tags ORDER BY id`)
	if err != nil {
		fmt.Printf("tags: query failed: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("=== tags ===")
	n := 0
	for rows.Next() {
		var id int64
		var name, slug sql.NullString
		if err := rows.Scan(&id, &name, &slug); err != nil {
			fmt.Printf("tags: scan failed: %v\n", err)
			return
		}
		n++
		fmt.Printf("  #%d name=%q slug=%q\n", id, name.String, slug.String)
	}
	if n == 0 {
		fmt.Println("  (no rows)")
	}
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "dbdump: %s: %v\n", what, err)
	os.Exit(1)
}
