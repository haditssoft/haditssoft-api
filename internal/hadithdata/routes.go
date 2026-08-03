package hadithdata

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(rg fiber.Router, handler *Handler) {
	rg.Get("/loadMainData/:kitabName/:number", handler.MainData)
	rg.Get("/classificationData/:kitabName/:number/:classify", handler.ClassificationData)
	rg.Get("/loadCustomData/:kitabName/:number/:position/:actionId", handler.CustomData)
	rg.Get("/loadSanadHadits/:kitabName/:number", handler.Sanad)
	rg.Get("/loadScholarComment/:narratorId", handler.ScholarComment)
	rg.Get("/loadTotalHadith/:kitabName/:narratorId", handler.TotalHadith)
	rg.Get("/loadSimilarHadith/:kitabName/:number", handler.SimilarHadith)
	rg.Get("/loadCompleteProfile/:narratorId", handler.NarratorCompleteProfile)
	rg.Get("/searchNoLain/:kitabName/:number", handler.OtherNumber)
	rg.Get("/loadBiographyData/:kitabName", handler.Biography)
	rg.Get("/loadAllBooks/:kitabName", handler.Book)
	rg.Get("/loadAllChapters/endfirst/:kitabName/:start/:vSelectedK", handler.ChapterEndFirst)
	rg.Get("/loadAllChapters/:kitabName/:start/:end", handler.ChapterList)
	rg.Post("/loadListOfRawiName/", handler.Narrator)
}
