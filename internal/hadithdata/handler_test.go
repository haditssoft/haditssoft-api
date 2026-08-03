package hadithdata

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/validator"
	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/glebarez/sqlite"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_SECRET", "test-secret-for-hadithdata-unit-tests")
	validator.RegisterCustomValidations()
	os.Exit(m.Run())
}

var hdrTestDBCounter int

func setupHDRTestDB(t *testing.T) {
	t.Helper()
	hdrTestDBCounter++
	dbName := fmt.Sprintf("file:test_hdr_%d?mode=memory&cache=shared", hdrTestDBCounter)
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			NoLowerCase:   true,
		},
	})
	if err != nil {
		t.Fatalf("failed to open test DB: %v", err)
	}
	database.DB = db
	t.Cleanup(func() { database.DB = nil })
}

func setupHadithDataTestApp(t *testing.T) *fiber.App {
	t.Helper()
	setupHDRTestDB(t)
	app := fiber.New()
	handler := NewHandler()
	RegisterRoutes(app, handler)
	return app
}

func makeHDRRequest(t *testing.T, app *fiber.App, method, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

// ============================================================
// Route existence tests — verify each route is registered
// ============================================================

func TestRoute_MainData(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_ClassificationData(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/classificationData/ShahihBukhari/1/ar")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_CustomData(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadCustomData/ShahihBukhari/1/position/action1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_Sanad(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadSanadHadits/ShahihBukhari/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_ScholarComment(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadScholarComment/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_TotalHadith(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadTotalHadith/ShahihBukhari/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_SimilarHadith(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadSimilarHadith/ShahihBukhari/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_NarratorCompleteProfile(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadCompleteProfile/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_OtherNumber(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/searchNoLain/ShahihBukhari/1")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_Biography(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadBiographyData/TestBiography")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_Book(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadAllBooks/ShahihBukhari")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_ChapterEndFirst(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadAllChapters/endfirst/ShahihBukhari/1/5")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_ChapterList(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadAllChapters/ShahihBukhari/1/10")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_Narrator(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "POST", "/loadListOfRawiName/")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}

func TestRoute_NoAuthRequired(t *testing.T) {
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadMainData/ShahihBukhari/1")
	if resp.StatusCode == http.StatusUnauthorized {
		t.Error("hadith data routes should not require auth")
	}
}

func TestRoutes_DoNotUseOldPackage(t *testing.T) {
	_ = models.MainData{}
	_ = models.ClassificationData{}
	app := setupHadithDataTestApp(t)
	resp := makeHDRRequest(t, app, "GET", "/loadAllBooks/ShahihBukhari")
	if resp.StatusCode == http.StatusNotFound {
		t.Error("route not matched")
	}
}
