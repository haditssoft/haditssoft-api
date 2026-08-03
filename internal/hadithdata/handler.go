package hadithdata

import (
	"net/url"

	"github.com/haditssoft/haditssoft-backend/models"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) MainData(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")

	result, err := models.LoadMainData(kitabName, number)
	if err != nil {
		return err
	}

	return c.JSON([]interface{}{[]models.MainData{result}, "MAINBOOKSDATA"})
}

func (h *Handler) ClassificationData(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")
	classify := c.Params("classify")

	result, err := models.LoadClassificationData(kitabName, classify, number)
	if err != nil {
		return err
	}

	return c.JSON([]interface{}{[]models.ClassificationData{result}, "CLASSIFICATIONDATA", number})
}

func (h *Handler) CustomData(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")
	position := c.Params("position")
	actionId := c.Params("actionId")

	result, err := models.LoadCustomData([]string{kitabName, number, position, actionId})
	if err != nil {
		return err
	}

	return c.JSON([]interface{}{[]models.CustomData{result}, actionId, position})
}

func (h *Handler) Sanad(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")

	result, err := models.GetSanad(kitabName, number)
	if err != nil {
		return err
	}

	return c.JSON([]interface{}{result, "SANADRESULT"})
}

func (h *Handler) ScholarComment(c *fiber.Ctx) error {
	narratorId := c.Params("narratorId")

	result, err := models.GetScholarComment(narratorId)
	if err != nil {
		return err
	}

	return c.JSON([]interface{}{result, "SCHOLARCOMMENT"})
}

func (h *Handler) TotalHadith(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")
	narratorId := c.Params("narratorId")

	hadithsNumber, err := models.GetTotalHadith(kitabName, narratorId)
	if err != nil {
		return err
	}

	if len(hadithsNumber) == 0 {
		return c.JSON(
			[]interface{}{
				hadithsNumber,
				"TOTALHADITHROWSRESULT",
				[]interface{}{
					[]models.ClassificationData{},
					"TOTALHADITHDATA",
					1,
				},
			},
		)
	}

	result, err := models.AfterGetTotalHadith(kitabName, hadithsNumber[0].NoHdt)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			hadithsNumber,
			"TOTALHADITHROWSRESULT",
			[]interface{}{
				[]models.ClassificationData{result},
				"TOTALHADITHDATA",
				1,
			},
		},
	)
}

func (h *Handler) SimilarHadith(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")
	number := c.Params("number")

	result, err := models.GetSimilarHadith(kitabName, number)
	if err != nil {
		return err
	}

	return c.JSON([]interface{}{result, "SIMILARHADITHRESULT"})
}

func (h *Handler) NarratorCompleteProfile(c *fiber.Ctx) error {
	narratorId := c.Params("narratorId")

	row, err := models.GetNarratorCompleteProfile(narratorId)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			[]*models.DaftarRawi{row},
			"COMPLETEPROFILERESULT",
		},
	)
}

func (h *Handler) OtherNumber(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")
	number := c.Params("number")

	originalNumber, err := models.GetOriginalNumber(kitabName, number)
	if err != nil {
		return err
	}

	result, err := models.LoadMainData(kitabName, originalNumber)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			[]models.MainData{result},
			"MAINBOOKSDATA",
		},
	)
}

func (h *Handler) Biography(c *fiber.Ctx) error {
	kitabName, err := url.QueryUnescape(c.Params("kitabName"))
	if err != nil {
		return err
	}

	result, err := models.GetBiography(kitabName)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			[]interface{}{result},
			kitabName,
		},
	)
}

func (h *Handler) Book(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")

	result, err := models.GetAllBooks(kitabName)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			result,
			"ALLBOOKSRESULT",
		},
	)
}

func (h *Handler) ChapterEndFirst(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")
	start := c.Params("start")
	vSelectedK := c.Params("vSelectedK")

	results, err := models.GetBeginingOfNextBookTitle(kitabName, start, vSelectedK)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			results,
			"ALLCHAPTERSRESULT",
		},
	)
}

func (h *Handler) ChapterList(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")
	start := c.Params("start")
	end := c.Params("end")

	results, err := models.GetAllChapters(kitabName, start, end)
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			results,
			"ALLCHAPTERSRESULT",
		},
	)
}

type narratorFilter struct {
	Name     string
	Kunyah   string
	Classify string
	Level    string
}

func (h *Handler) Narrator(c *fiber.Ctx) error {
	var filter narratorFilter
	if err := c.BodyParser(&filter); err != nil {
		return err
	}

	rows, err := models.GetNarrators([]string{filter.Name, filter.Kunyah, filter.Classify, filter.Level})
	if err != nil {
		return err
	}

	return c.JSON(
		[]interface{}{
			rows,
			"LISTOFNARRATORNAMERESULT",
		},
	)
}
