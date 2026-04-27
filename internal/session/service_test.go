package session

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

type fakeStudentDataClient struct {
	calls int
	data  vt.StudentData
	err   error
}

func (f *fakeStudentDataClient) StudentData(ctx context.Context, authtoken string) (vt.StudentData, error) {
	f.calls++
	return f.data, f.err
}

func TestImportValidSession(t *testing.T) {
	db := newServiceTestDB(t)
	client := &fakeStudentDataClient{data: studentData("person", "fresh-proof")}
	now := fixedNow()
	svc := Service{DB: db, VTClient: client, Now: func() time.Time { return now }}

	got, err := svc.Import(context.Background(), CapturedCredentials{
		CapturedAt: now.Add(-time.Hour),
		Authtoken:  "token",
	})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if client.calls != 1 {
		t.Fatalf("StudentData calls = %d, want 1", client.calls)
	}
	if got.Status != StatusValid || !got.LastValidatedAt.Valid {
		t.Fatalf("unexpected stored session: %+v", got)
	}
	if got.Authtoken != "token" || got.PersID != "person" || got.PersIDProof != "fresh-proof" {
		t.Fatalf("unexpected credentials in session: %+v", got)
	}
}

func TestImportUsesNowWhenCapturedAtMissing(t *testing.T) {
	db := newServiceTestDB(t)
	now := fixedNow()
	svc := Service{
		DB:       db,
		VTClient: &fakeStudentDataClient{data: studentData("person", "proof")},
		Now:      func() time.Time { return now },
	}

	got, err := svc.Import(context.Background(), CapturedCredentials{
		Authtoken: "token",
	})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if !got.CapturedAt.Equal(now) {
		t.Fatalf("CapturedAt = %s, want %s", got.CapturedAt, now)
	}
}

func TestImportValidationErrorsDoNotSave(t *testing.T) {
	tests := []struct {
		name    string
		payload CapturedCredentials
		client  *fakeStudentDataClient
	}{
		{name: "missing authtoken", payload: CapturedCredentials{}, client: &fakeStudentDataClient{data: studentData("person", "proof")}},
		{name: "missing studentdata pers id", payload: validPayload(), client: &fakeStudentDataClient{data: studentData("", "proof")}},
		{name: "missing fresh pers proof", payload: validPayload(), client: &fakeStudentDataClient{data: studentData("person", "")}},
		{name: "vt error", payload: validPayload(), client: &fakeStudentDataClient{err: errors.New("upstream failed")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newServiceTestDB(t)
			svc := Service{DB: db, VTClient: tt.client, Now: fixedNow}
			_, err := svc.Import(context.Background(), tt.payload)
			if err == nil {
				t.Fatal("Import returned nil error")
			}
			if strings.Contains(err.Error(), "raw-token-secret") ||
				strings.Contains(err.Error(), "raw-person-secret") ||
				strings.Contains(err.Error(), "raw-proof-secret") {
				t.Fatalf("error leaked credentials: %v", err)
			}
			if _, currentErr := store.CurrentSession(context.Background(), db); !errors.Is(currentErr, store.ErrNoSession) {
				t.Fatalf("CurrentSession error = %v, want ErrNoSession", currentErr)
			}
		})
	}
}

func TestImportMissingFieldsDoNotCallVT(t *testing.T) {
	client := &fakeStudentDataClient{data: studentData("person", "proof")}
	svc := Service{DB: newServiceTestDB(t), VTClient: client, Now: fixedNow}
	_, err := svc.Import(context.Background(), CapturedCredentials{})
	if err == nil {
		t.Fatal("Import returned nil error")
	}
	if client.calls != 0 {
		t.Fatalf("StudentData calls = %d, want 0", client.calls)
	}
}

func TestImportIgnoresLegacyCapturedIdentityFields(t *testing.T) {
	db := newServiceTestDB(t)
	svc := Service{
		DB:       db,
		VTClient: &fakeStudentDataClient{data: studentData("studentdata-person", "studentdata-proof")},
		Now:      fixedNow,
	}

	got, err := svc.Import(context.Background(), CapturedCredentials{
		Authtoken:   "token",
		PersID:      "stale-captured-person",
		PersIDProof: "stale-captured-proof",
	})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if got.PersID != "studentdata-person" || got.PersIDProof != "studentdata-proof" {
		t.Fatalf("stored legacy captured identity instead of studentdata identity: %+v", got)
	}
}

func TestValidateCurrentValid(t *testing.T) {
	db := newServiceTestDB(t)
	now := fixedNow()
	saveStoredSession(t, db, "unknown", sql.NullTime{})
	svc := Service{
		DB:       db,
		VTClient: &fakeStudentDataClient{data: studentData("person", "fresh-proof")},
		Now:      func() time.Time { return now },
	}

	result, err := svc.ValidateCurrent(context.Background())
	if err != nil {
		t.Fatalf("ValidateCurrent returned error: %v", err)
	}
	if result.ValidationErr != nil {
		t.Fatalf("ValidateCurrent validation error = %v, want nil", result.ValidationErr)
	}
	got := result.Session
	if got.Status != StatusValid || !got.LastValidatedAt.Valid {
		t.Fatalf("unexpected session: %+v", got)
	}
	if got.PersIDProof != "fresh-proof" {
		t.Fatalf("PersIDProof = %q, want fresh-proof", got.PersIDProof)
	}
}

func TestValidateCurrentInvalidOnVTError(t *testing.T) {
	db := newServiceTestDB(t)
	now := fixedNow()
	saveStoredSession(t, db, "valid", sql.NullTime{})
	svc := Service{
		DB:       db,
		VTClient: &fakeStudentDataClient{err: errors.New("upstream failed")},
		Now:      func() time.Time { return now },
	}

	result, err := svc.ValidateCurrent(context.Background())
	if err != nil {
		t.Fatalf("ValidateCurrent returned error: %v", err)
	}
	if result.ValidationErr == nil {
		t.Fatal("ValidateCurrent returned nil validation error")
	}
	got := result.Session
	if got.Status != StatusInvalid || !got.LastValidatedAt.Valid {
		t.Fatalf("unexpected invalid session: %+v", got)
	}
}

func TestValidateCurrentInvalidOnMismatch(t *testing.T) {
	db := newServiceTestDB(t)
	saveStoredSession(t, db, "valid", sql.NullTime{})
	svc := Service{
		DB:       db,
		VTClient: &fakeStudentDataClient{data: studentData("other", "proof")},
		Now:      fixedNow,
	}

	result, err := svc.ValidateCurrent(context.Background())
	if err != nil {
		t.Fatalf("ValidateCurrent returned error: %v", err)
	}
	if result.ValidationErr == nil {
		t.Fatal("ValidateCurrent returned nil validation error")
	}
	got := result.Session
	if got.Status != StatusInvalid {
		t.Fatalf("Status = %q, want invalid", got.Status)
	}
}

func TestValidateCurrentInvalidOnMissingFreshProof(t *testing.T) {
	db := newServiceTestDB(t)
	saveStoredSession(t, db, "valid", sql.NullTime{})
	svc := Service{
		DB:       db,
		VTClient: &fakeStudentDataClient{data: studentData("person", "")},
		Now:      fixedNow,
	}

	result, err := svc.ValidateCurrent(context.Background())
	if err != nil {
		t.Fatalf("ValidateCurrent returned error: %v", err)
	}
	if result.ValidationErr == nil {
		t.Fatal("ValidateCurrent returned nil validation error")
	}
	got := result.Session
	if got.Status != StatusInvalid {
		t.Fatalf("Status = %q, want invalid", got.Status)
	}
}

func validPayload() CapturedCredentials {
	return CapturedCredentials{
		Authtoken: "raw-token-secret",
	}
}

func studentData(persID, proof string) vt.StudentData {
	return vt.StudentData{Pers: vt.Person{ID: persID, IDProof: proof}}
}

func fixedNow() time.Time {
	return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
}

func saveStoredSession(t *testing.T, db *sql.DB, status string, lastValidatedAt sql.NullTime) {
	t.Helper()
	if _, err := store.SaveSession(context.Background(), db, store.Session{
		Authtoken:       "token",
		PersID:          "person",
		PersIDProof:     "proof",
		CapturedAt:      fixedNow().Add(-time.Hour),
		LastValidatedAt: lastValidatedAt,
		Status:          status,
	}); err != nil {
		t.Fatalf("SaveSession returned error: %v", err)
	}
}

func newServiceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return db
}
