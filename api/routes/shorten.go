package routes

import (
	"os"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/go-redis/redis/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/razyneko/url-shortener-go-fiber-redis/database"
	"github.com/razyneko/url-shortener-go-fiber-redis/helpers"
)

// by defining request and response using structs makes code stable, thers a format frontend can expect the rq or res to be in

type request struct {
	URL				string			`json:"url"` //serialization
	CustomShort		string			`json:"short"`
	Expiry			time.Duration	`json:"expiry"`
}

type response struct{
	URL					string				`json:"url"`
	CustomShort			string				`json:"short"`
	Expiry				time.Duration		`json:"expiry"`
	XRateRemaining		int					`json:"rate_limit"`
	XRateLimitReset		time.Duration		`json:"rate-limit-reset"`
}

func ShortenURL(c *fiber.Ctx) error {
	body := new(request)

	// use parser for json to struct

	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cannot parse JSON"})
	}

	// implement rate limiting
	r2 := database.CreateClient(1)
	defer r2.Close() //closing connection to db
	// redis is a key value pair database
	val, err := r2.Get(database.Ctx, c.IP()).Result() // IP is the key in this case

	if err == redis.Nil {
		// if user's ip add not in db, set api quota reset time 
		_ = r2.Set(database.Ctx, c.IP(), os.Getenv("API_QUOTA"), 30*60*time.Second).Err() 
	} else {
		valInt, _ := strconv.Atoi(val)
		if valInt <= 0 {
			limit, _ := r2.TTL(database.Ctx, c.IP()).Result()
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "rate limit exceeded",
				"rate_limit_rest": limit / time.Nanosecond / time.Minute,
			})
		}
	}

	// check if input is an actual url
	if !govalidator.IsURL(body.URL){
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error" : "Invalid URL"})
	}

	// if user enters localhost:3000 to be shortened our program goes into infinite loop

	// check for domain error
	if !helpers.RemoveDomainError(body.URL){
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "This domain can't be shortened"})
	}

	// enforce https, SSL
	body.URL = helpers.EnforceHTTP(body.URL)

	var id string

	if body.CustomShort == "" {
		id = uuid.New().String()[:6]
	 }else {
		id = body.CustomShort
	 }

	r := database.CreateClient(0)
	defer r.Close()

	val, _ = r.Get(database.Ctx, id).Result()
	if val != "" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":"URL custom short is already in use",
		})
	}

	if body.Expiry == 0 {
		body.Expiry = 24
	}

	err = r.Set(database.Ctx, id, body.URL, body.Expiry*3600*time.Second).Err()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error" : "Unable to connect to server",
		})
	}

	resp := response {
		URL:				body.URL,
		CustomShort:		"",
		Expiry:				body.Expiry,
		XRateRemaining:		10,
		XRateLimitReset:	30,
	}

	r2.Decr(database.Ctx, c.IP())

	val, _ = r2.Get(database.Ctx, c.IP()).Result()
	resp.XRateRemaining, _ = strconv.Atoi(val)
	ttl, _ := r2.TTL(database.Ctx, c.IP()).Result()
	resp.XRateLimitReset = ttl / time.Nanosecond / time.Minute

	resp.CustomShort = os.Getenv("DOMAIN") + "/" + id
	return c.Status(fiber.StatusOK).JSON(resp)
}