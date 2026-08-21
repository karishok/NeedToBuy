package child

import (
	"context"
	"errors"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/dbtest"
)

func mustCreateParent(t *testing.T, ctx context.Context, db auth.Querier, email string) int64 {
	t.Helper()
	var id int64
	if err := db.GetContext(ctx, &id, `INSERT INTO users (email) VALUES ($1) RETURNING id`, email); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	return id
}

func TestCreateChild_ThenListChildren_ReturnsIt(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentID := mustCreateParent(t, ctx, tx, "parent@example.com")

	created, err := createChild(ctx, tx, parentID, "Тимофей", time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}
	if created.Name != "Тимофей" {
		t.Fatalf("Name = %q, want Тимофей", created.Name)
	}
	if created.PublicShareToken == "" {
		t.Fatal("PublicShareToken is empty, want a generated token")
	}

	rows, err := listChildren(ctx, tx, parentID)
	if err != nil {
		t.Fatalf("listChildren() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != created.ID {
		t.Fatalf("listChildren() = %+v, want a single row matching id %d", rows, created.ID)
	}
}

func TestListChildren_IsolatesParents(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentA := mustCreateParent(t, ctx, tx, "a@example.com")
	parentB := mustCreateParent(t, ctx, tx, "b@example.com")

	if _, err := createChild(ctx, tx, parentA, "Child A", time.Now().AddDate(-2, 0, 0)); err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	rows, err := listChildren(ctx, tx, parentB)
	if err != nil {
		t.Fatalf("listChildren() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("listChildren() for parentB = %+v, want empty", rows)
	}
}

func TestUpdateChild_ChangesNameAndBirthDate(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentID := mustCreateParent(t, ctx, tx, "parent@example.com")
	created, err := createChild(ctx, tx, parentID, "Тимофей", time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	newName := "Тимур"
	newBirthDate := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated, err := updateChild(ctx, tx, parentID, created.ID, &newName, &newBirthDate)
	if err != nil {
		t.Fatalf("updateChild() error = %v", err)
	}
	if updated.Name != "Тимур" {
		t.Fatalf("Name = %q, want Тимур", updated.Name)
	}
	if !updated.BirthDate.Equal(newBirthDate) {
		t.Fatalf("BirthDate = %v, want %v", updated.BirthDate, newBirthDate)
	}
}

func TestUpdateChild_WrongParent_ReturnsErrNotFound(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentA := mustCreateParent(t, ctx, tx, "a@example.com")
	parentB := mustCreateParent(t, ctx, tx, "b@example.com")
	created, err := createChild(ctx, tx, parentA, "Child A", time.Now().AddDate(-2, 0, 0))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	newName := "Hijacked"
	_, err = updateChild(ctx, tx, parentB, created.ID, &newName, nil)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("updateChild() error = %v, want errNotFound", err)
	}
}

func TestDeleteChild_ThenListChildren_IsEmpty(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentID := mustCreateParent(t, ctx, tx, "parent@example.com")
	created, err := createChild(ctx, tx, parentID, "Тимофей", time.Now().AddDate(-2, 0, 0))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	if err := deleteChild(ctx, tx, parentID, created.ID); err != nil {
		t.Fatalf("deleteChild() error = %v", err)
	}

	rows, err := listChildren(ctx, tx, parentID)
	if err != nil {
		t.Fatalf("listChildren() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("listChildren() after delete = %+v, want empty", rows)
	}
}

func TestDeleteChild_WrongParent_ReturnsErrNotFound(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentA := mustCreateParent(t, ctx, tx, "a@example.com")
	parentB := mustCreateParent(t, ctx, tx, "b@example.com")
	created, err := createChild(ctx, tx, parentA, "Child A", time.Now().AddDate(-2, 0, 0))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	err = deleteChild(ctx, tx, parentB, created.ID)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("deleteChild() error = %v, want errNotFound", err)
	}
}

func TestValidateName(t *testing.T) {
	if _, err := validateName("  "); err == nil {
		t.Error("validateName(\"  \") error = nil, want error for blank name")
	}
	if _, err := validateName(""); err == nil {
		t.Error("validateName(\"\") error = nil, want error for empty name")
	}
	got, err := validateName("  Тимофей  ")
	if err != nil {
		t.Fatalf("validateName() error = %v", err)
	}
	if got != "Тимофей" {
		t.Fatalf("validateName() = %q, want trimmed \"Тимофей\"", got)
	}
	tooLong := make([]byte, 101)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if _, err := validateName(string(tooLong)); err == nil {
		t.Error("validateName() with 101 chars error = nil, want error")
	}
}

func TestParseBirthDate(t *testing.T) {
	if _, err := parseBirthDate("not-a-date"); err == nil {
		t.Error("parseBirthDate(\"not-a-date\") error = nil, want error")
	}
	future := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	if _, err := parseBirthDate(future); err == nil {
		t.Error("parseBirthDate() with future date error = nil, want error")
	}
	got, err := parseBirthDate("2024-03-15")
	if err != nil {
		t.Fatalf("parseBirthDate() error = %v", err)
	}
	want := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseBirthDate() = %v, want %v", got, want)
	}
}
