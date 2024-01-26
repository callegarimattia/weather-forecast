package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/jmoiron/sqlx"
)

type GeoResponse struct {
	Results []LatLong `json:"results"`
}

type LatLong struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func fetchLatLong(city string) (*LatLong, error) {
	// Make the request
	endpoint := fmt.Sprintf("https://geocoding-api.open-meteo.com/v1/search?name=%s&count=10&language=en&format=json", url.QueryEscape(city))
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("error in request to geocoding-api: %w", err)
	}
	defer resp.Body.Close()
	// Decode the response to json
	var response GeoResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding response to json: %w", err)
	}
	if len(response.Results) < 1 {
		return nil, errors.New("the decoded json is empty")
	}
	return &response.Results[0], nil
}

func getLatLong(db *sqlx.DB, name string) (*LatLong, error) {
	var latLong *LatLong
	err := db.Get(&latLong, "SELECT lat, long FROM cities WHERE name = $1", name)
	if err == nil {
		return latLong, nil
	}

	latLong, err = fetchLatLong(name)
	if err != nil {
		return nil, err
	}

	err = insertCity(db, name, *latLong)
	if err != nil {
		return nil, err
	}

	return latLong, nil
}

func getWeather(latLong LatLong) (string, error) {
	// Make the request
	endpoint := fmt.Sprintf("https://api.open-meteo.com/v1/forecast?latitude=%.6f&longitude=%.6f&hourly=temperature_2m", latLong.Latitude, latLong.Longitude)
	resp, err := http.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("error in request to api.open-meteo: %w", err)
	}
	defer resp.Body.Close()
	// Decode the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading the body from the response: %w", err)
	}
	return string(body), nil
}

func extractWeatherData(city string, rawWeather string) (WeatherDisplay, error) {
	var weatherResponse WeatherResponse
	if err := json.Unmarshal([]byte(rawWeather), &weatherResponse); err != nil {
		return WeatherDisplay{}, fmt.Errorf("error decoding weather response: %w", err)
	}

	var forecasts []Forecast
	for i, t := range weatherResponse.Hourly.Time {
		date, err := time.Parse("2006-01-02T15:04", t)
		if err != nil {
			return WeatherDisplay{}, fmt.Errorf("error parsing time: %w", err)
		}
		forecast := Forecast{
			Date:        date.Format("Mon 15:04"),
			Temperature: fmt.Sprintf("%.1f°C", weatherResponse.Hourly.Temperature2m[i]),
		}
		forecasts = append(forecasts, forecast)
	}
	return WeatherDisplay{
		City:      city,
		Forecasts: forecasts,
	}, nil
}

func insertCity(db *sqlx.DB, name string, latLong LatLong) error {
	_, err := db.Exec("INSERT INTO cities (name, lat, long) VALUES ($1, $2, $3)", name, latLong.Latitude, latLong.Longitude)
	return err
}

func getLastCities(db *sqlx.DB) ([]string, error) {
	var cities []string
	err := db.Select(&cities, "SELECT name FROM cities ORDER BY id DESC LIMIT 20")
	if err != nil {
		return nil, err
	}
	return cities, nil
}
