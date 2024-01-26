package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type WeatherResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
	Hourly    struct {
		Time          []string  `json:"time"`
		Temperature2m []float64 `json:"temperature_2m"`
	} `json:"hourly"`
}

type WeatherDisplay struct {
	City      string
	Forecasts []Forecast
}

type Forecast struct {
	Date        string
	Temperature string
}

func main() {
	server := gin.Default()
	server.LoadHTMLGlob("views/*")
	db := sqlx.MustConnect("postgres", os.Getenv("DATABASE_URL"))

	server.GET("/weather", func(ctx *gin.Context) {
		city := ctx.Query("city")
		latLong, err := getLatLong(db, city)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		weather, err := getWeather(*latLong)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		weatherDisplay, err := extractWeatherData(city, weather)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		ctx.HTML(http.StatusOK, "weather.html", weatherDisplay)
	})

	server.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.html", nil)
	})

	server.GET("/stats", gin.BasicAuth(gin.Accounts{
		"forecast": "forecast",
	}), func(c *gin.Context) {
		cities, err := getLastCities(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.HTML(http.StatusOK, "stats.html", cities)
	})

	server.Run()
}
