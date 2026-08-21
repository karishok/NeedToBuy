// Package child implements CRUD for a parent's child profiles: name,
// birth date, the public share token a future wishlist share link will
// use, and consent to processing the child's personal data (152-ФЗ).
package child

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"needtobuy/internal/auth"
)

// errNotFound signals no matching child — missing, or owned by someone
// else. Handlers map it to a 404 without distinguishing the two cases,
// so a non-owner can't tell which one it is.
var errNotFound = errors.New("child: not found")

// row mirrors one row of the children table as scanned from Postgres.
type row struct {
	ID               int64     `db:"id"`
	ParentID         int64     `db:"parent_id"`
	Name             string    `db:"name"`
	BirthDate        time.Time `db:"birth_date"`
	PublicShareToken string    `db:"public_share_token"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

const childColumns = "id, parent_id, name, birth_date, public_share_token, created_at, updated_at"

// generateShareToken returns a random opaque token for a child's public
// wishlist link, used directly as public_share_token.
func generateShareToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("child: generate share token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// createChild inserts a new child profile for parentID and returns it.
// Consent is assumed already validated true by the caller; this function
// always stamps consent_child_data_at to now().
func createChild(ctx context.Context, db auth.Querier, parentID int64, name string, birthDate time.Time) (row, error) {
	token, err := generateShareToken()
	if err != nil {
		return row{}, err
	}
	var r row
	err = db.GetContext(ctx, &r, `
		INSERT INTO children (parent_id, name, birth_date, public_share_token, consent_child_data_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING `+childColumns,
		parentID, name, birthDate, token)
	if err != nil {
		return row{}, fmt.Errorf("child: create: %w", err)
	}
	return r, nil
}

// listChildren returns all children belonging to parentID, oldest first.
func listChildren(ctx context.Context, db auth.Querier, parentID int64) ([]row, error) {
	var rows []row
	if err := db.SelectContext(ctx, &rows, `
		SELECT `+childColumns+`
		FROM children WHERE parent_id = $1 ORDER BY created_at`, parentID); err != nil {
		return nil, fmt.Errorf("child: list: %w", err)
	}
	return rows, nil
}

// getChild returns a single child owned by parentID. Returns errNotFound
// if it doesn't exist or belongs to someone else.
func getChild(ctx context.Context, db auth.Querier, parentID, id int64) (row, error) {
	var r row
	err := db.GetContext(ctx, &r, `
		SELECT `+childColumns+`
		FROM children WHERE id = $1 AND parent_id = $2`, id, parentID)
	if errors.Is(err, sql.ErrNoRows) {
		return row{}, errNotFound
	}
	if err != nil {
		return row{}, fmt.Errorf("child: get: %w", err)
	}
	return r, nil
}

// updateChild applies a partial update (name and/or birth date, whichever
// is non-nil) to a child owned by parentID, and returns the updated row.
// Returns errNotFound if id doesn't belong to parentID (or doesn't exist).
func updateChild(ctx context.Context, db auth.Querier, parentID, id int64, name *string, birthDate *time.Time) (row, error) {
	setClauses := []string{"updated_at = now()"}
	args := []any{}
	argN := 1
	if name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argN))
		args = append(args, *name)
		argN++
	}
	if birthDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("birth_date = $%d", argN))
		args = append(args, *birthDate)
		argN++
	}
	args = append(args, id, parentID)
	query := fmt.Sprintf(`UPDATE children SET %s WHERE id = $%d AND parent_id = $%d`,
		strings.Join(setClauses, ", "), argN, argN+1)

	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return row{}, fmt.Errorf("child: update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return row{}, fmt.Errorf("child: update rows affected: %w", err)
	}
	if n == 0 {
		return row{}, errNotFound
	}
	return getChild(ctx, db, parentID, id)
}

// deleteChild removes a child owned by parentID. Returns errNotFound if
// it doesn't exist or belongs to someone else.
func deleteChild(ctx context.Context, db auth.Querier, parentID, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM children WHERE id = $1 AND parent_id = $2`, id, parentID)
	if err != nil {
		return fmt.Errorf("child: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("child: delete rows affected: %w", err)
	}
	if n == 0 {
		return errNotFound
	}
	return nil
}
