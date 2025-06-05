package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	_ "github.com/lib/pq"
)

type DiscoverMoviesResponse struct {
	Page       int             `json:"page"`
	TotalPages int             `json:"total_pages"`
	Results    []MovieResponse `json:"results"`
}

type MovieResponse struct {
	Id          int    `json:"id"`
	Title       string `json:"original_title"`
	ReleaseDate string `json:"release_date"`
	Adult       bool   `json:"adult"`
	PosterPath  string `json:"poster_path"`
	GenreIds    []int  `json:"genre_ids"`
	Overview    string `json:"overview"`
}

type MovieDetailsResponse struct {
	Id          int    `json:"id"`
	Title       string `json:"original_title"`
	ReleaseDate string `json:"release_date"`
	Adult       bool   `json:"adult"`
	PosterPath  string `json:"poster_path"`
	GenreIds    []struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
	} `json:"genres"`
	Overview string `json:"overview"`
}

type ConfigResponse struct {
	Images struct {
		SecureBaseUrl string `json:"secure_base_url"`
	} `json:"images"`
}

var apiKey = "eyJhbGciOiJIUzI1NiJ9.eyJhdWQiOiIwYzFhNmQ5YWE2YTczMmE2NWY3MzA5OTM5NWE0MDIyNiIsIm5iZiI6MTc0ODYwMTMzMy45NjUsInN1YiI6IjY4Mzk4OWY1OGFlMzI3OWUxNDhmZTY2MSIsInNjb3BlcyI6WyJhcGlfcmVhZCJdLCJ2ZXJzaW9uIjoxfQ.6t20bvZNk3Qdmf4uySyJH1eCwa_NmPOGtkExV1tU2_U"

// checks if str is mostly unicode (at least 90%)
// this is needed to exclude movies with weird names labeled as english
func isMostlyEnglish(str string) bool {
	latinCount := 0
	total := 0
	for _, r := range str {
		if unicode.IsLetter(r) {
			total++
			if r <= 0x024F { // Covers Basic Latin + Latin-1 Supplement + Latin Extended-A/B (basic European)
				latinCount++
			}
		}
	}
	return total == 0 || float64(latinCount)/float64(total) > 0.9
}

// checks if a movie title contains inappropriate content
func containsInappropriateContent(title string) bool {
	// Convert to lowercase for case-insensitive matching
	lowerTitle := strings.ToLower(title)

	// List of inappropriate keywords to filter out
	inappropriateKeywords := []string{
		"cock", "dick", "penis", "pussy", "vagina", "sex", "porn", "xxx",
		"erotic", "adult", "mature", "explicit", "nude", "naked", "strip",
		"lesbian", "gay", "anal", "oral", "orgasm", "masturbat", "fetish",
		"bdsm", "kinky", "horny", "slutty", "whore", "fuck", "shit",
		"milf", "gilf", "teen", "barely", "virgin", "escort", "prostitut",
		"threesome", "gangbang", "cumshot", "blowjob", "handjob", "footjob",
	}

	for _, keyword := range inappropriateKeywords {
		if strings.Contains(lowerTitle, keyword) {
			return true
		}
	}

	return false
}

// Function to get movie details including overview
func getMovieDetails(movieId int, posterBaseUrl string) (MovieResponse, error) {
	detailsUrl := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d", movieId)
	req, err := http.NewRequest("GET", detailsUrl, nil)
	if err != nil {
		return MovieResponse{}, err
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+apiKey)

	// Add a small delay to respect rate limits
	time.Sleep(100 * time.Millisecond)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return MovieResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return MovieResponse{}, fmt.Errorf("bad status: %s", res.Status)
	}

	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return MovieResponse{}, err
	}

	var details MovieDetailsResponse
	err = json.Unmarshal(bytes, &details)
	if err != nil {
		return MovieResponse{}, err
	}

	// Convert to our MovieResponse format
	movie := MovieResponse{
		Id:          details.Id,
		Title:       details.Title,
		ReleaseDate: details.ReleaseDate,
		Adult:       details.Adult,
		PosterPath:  posterBaseUrl + "w342" + details.PosterPath,
		Overview:    details.Overview,
	}

	// Convert genres to genre IDs
	for _, genre := range details.GenreIds {
		movie.GenreIds = append(movie.GenreIds, genre.Id)
	}

	return movie, nil
}

func main() {

	// some movies are repeated in api response, so we need to store them by id
	movies := make(map[int]MovieResponse)

	// get the base url for posters
	configUrl := "https://api.themoviedb.org/3/configuration"
	req, err := http.NewRequest("GET", configUrl, nil)
	if err != nil {
		log.Fatal("Error when making request: ", err)
	}
	req.Header.Add("accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+apiKey)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal("Error when sending get request: ", err)
	}
	if res.StatusCode != 200 {
		log.Fatal("Bad status: ", res.Status)
	}
	bytes, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal("Error reading bytes from response: ", err)
	}
	res.Body.Close()

	config := ConfigResponse{}
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		log.Fatal("Error parsing json: ", err)
	}
	posterBaseUrl := config.Images.SecureBaseUrl

	// getting movies from discover endpoint first
	discoverUrl := "https://api.themoviedb.org/3/discover/movie"
	var movieIds []int

	for page := 1; page <= 5; page++ { // Reduced pages to avoid too many API calls
		req, err := http.NewRequest("GET", discoverUrl, nil)
		if err != nil {
			log.Fatal(err)
		}
		req.Header.Add("accept", "application/json")
		req.Header.Add("Authorization", "Bearer "+apiKey)
		q := req.URL.Query()
		q.Add("include_video", "false")
		q.Add("language", "en-US")
		q.Add("sort_by", "popularity.desc")
		q.Add("include_adult", "false") // Filter out adult movies
		q.Add("page", strconv.Itoa(page))
		req.URL.RawQuery = q.Encode()

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		if res.StatusCode != 200 {
			log.Println("Bad code ", res.Status)
			log.Println("Url: ", res.Request.URL.String())
			return
		}
		defer res.Body.Close()

		bytes, err := io.ReadAll(res.Body)
		if err != nil {
			log.Fatal(err)
		}
		content := DiscoverMoviesResponse{}
		json.Unmarshal(bytes, &content)
		fmt.Printf("PAGE %d - %d movies\n", page, len(content.Results))

		for _, m := range content.Results {
			// Skip adult movies as an additional filter
			if m.Adult {
				continue
			}
			// Skip movies with inappropriate content in title
			if containsInappropriateContent(m.Title) {
				fmt.Printf("Filtered out inappropriate movie: %s\n", m.Title)
				continue
			}
			// Skip non-English titles
			if !isMostlyEnglish(m.Title) {
				continue
			}

			movieIds = append(movieIds, m.Id)
		}
	}

	// Now get detailed information for each movie
	fmt.Printf("Fetching details for %d movies...\n", len(movieIds))
	for i, movieId := range movieIds {
		fmt.Printf("Fetching movie %d/%d (ID: %d)\n", i+1, len(movieIds), movieId)

		movie, err := getMovieDetails(movieId, posterBaseUrl)
		if err != nil {
			fmt.Printf("Error fetching details for movie %d: %v\n", movieId, err)
			continue
		}

		// Double-check filters with detailed data
		if movie.Adult || containsInappropriateContent(movie.Title) || !isMostlyEnglish(movie.Title) {
			continue
		}

		movies[movie.Id] = movie
	}

	// connect with bd(temporarly)
	connStr := "user=postgres password=YOUR_PASSWORD dbname=project_go sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	// clearing movies table
	deleteQuery := "DELETE FROM movies;"
	result, err := tx.Exec(deleteQuery)
	if err != nil {
		log.Fatal("Error deleting movies: ", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Deleted %d movies\n", rowsAffected)

	// Insert movies using prepared statement (safer and handles special characters better)
	insertQuery := "INSERT INTO movies(id, title, release_date, genre_ids, poster_url, description) VALUES ($1, $2, $3, $4, $5, $6)"
	stmt, err := tx.Prepare(insertQuery)
	if err != nil {
		log.Fatal("Error preparing statement: ", err)
	}
	defer stmt.Close()

	insertedCount := 0
	for _, m := range movies {
		genreString := ""
		for i, gId := range m.GenreIds {
			genreString += strconv.Itoa(gId)
			if i != len(m.GenreIds)-1 {
				genreString += ","
			}
		}

		_, err = stmt.Exec(m.Id, m.Title, m.ReleaseDate, genreString, m.PosterPath, m.Overview)
		if err != nil {
			fmt.Printf("Error inserting movie %s: %v\n", m.Title, err)
			continue
		}
		insertedCount++
	}

	fmt.Printf("Inserted %d movies\n", insertedCount)
	err = tx.Commit()
	if err != nil {
		log.Fatal("Error when committing transaction: ", err)
	}

	fmt.Println("Done!")
}
