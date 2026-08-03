package hadithadmin

import (
	"github.com/haditssoft/haditssoft-backend/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

func GetList(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))
	search := c.Query("search", "")

	results, total, err := models.SearchMainData(kitabName, page, limit, search)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data":  results,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func GetOne(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")

	result, err := models.AdminGetOne(kitabName, number)
	if err != nil {
		return err
	}

	return c.JSON(result)
}

func PostOne(c *fiber.Ctx) error {
	kitabName := c.Params("kitabName")

	var data models.AdminMainData
	if err := c.BodyParser(&data); err != nil {
		return err
	}

	result, err := models.AdminPostOne(kitabName, data)
	if err != nil {
		return err
	}

	return c.JSON(result)
}

func PutOne(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")

	var data models.AdminMainData
	if err := c.BodyParser(&data); err != nil {
		return err
	}

	result, err := models.AdminPutOne(kitabName, number, data)
	if err != nil {
		return err
	}

	return c.JSON(result)
}

func DeleteOne(c *fiber.Ctx) error {
	number := c.Params("number")
	kitabName := c.Params("kitabName")

	err := models.AdminDeleteOne(kitabName, number)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"message": "Record deleted successfully",
	})
}
