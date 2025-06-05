package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Movie struct {
	Id             int
	Name           string
	PosterUrl      string
	ReleaseDate    time.Time
	Genres         []int
	Description    string
	ReviewCount    int
	SelectedGenre  string
	SelectedYear   string
	SelectedRating string
}

type User struct {
	Id    int
	Login string
	Email string
	Phone string
}

type Review struct {
	Id        int
	MovieId   int
	UserId    int
	Rating    int
	Comment   string
	СreatedAt time.Time
}

type ReviewWithUser struct {
	Rating    int
	Comment   string
	CreatedAt time.Time
	UserLogin string
}

type DiscoverPageData struct {
	Movies      []Movie
	TopReviewed []Movie
	Newest      []Movie
	Genre       string
	Page        int
	GenreID     string
	Year        string
	Rating      string
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	RenderMainPage(w)
}

func DiscoverHanderl(w http.ResponseWriter, r *http.Request) {

	var genreMap = map[string]string{
		"28":    "Action",
		"12":    "Adventure",
		"16":    "Animation",
		"35":    "Comedy",
		"80":    "Crime",
		"99":    "Documentary",
		"18":    "Drama",
		"10751": "Family",
		"14":    "Fantasy",
		"27":    "Horror",
		"9648":  "Mystery",
		"10749": "Romance",
		"878":   "Science Fiction",
		"10770": "TV Movie",
		"53":    "Thriller",
		"10752": "War",
		"37":    "Western",
	}

	genreID := r.URL.Query().Get("genre")

	selectedGenre := ""
	if genreID != "" {
		if name, ok := genreMap[genreID]; ok {
			selectedGenre = name
		}
	}

	pageStr := r.URL.Query().Get("page")
	page := 1
	if pageStr != "" {
		parsed, err := strconv.Atoi(pageStr)
		if err == nil {
			page = parsed
		}
	}

	genreStr := r.URL.Query().Get("genre")
	yearStr := r.URL.Query().Get("year")
	ratingStr := r.URL.Query().Get("rating")
	var movies []Movie
	var err error

	if genreStr != "" && yearStr != "" {
		genreId, _ := strconv.Atoi(genreStr)
		year, _ := strconv.Atoi(yearStr)
		movies, err = GetMoviesByGenreAndYear(genreId, year)
	} else if genreStr != "" {
		genreId, _ := strconv.Atoi(genreStr)
		movies, err = GetMoviesByGenre(genreId)
	} else if yearStr != "" {
		year, _ := strconv.Atoi(yearStr)
		movies, err = GetMoviesByYear(year)
	} else {
		movies, err = GetMovies()
	}

	if err != nil {
		log.Println(err)
	}

	if ratingStr != "" {
		ratingThresholder, err := strconv.ParseFloat(ratingStr, 64)
		if err == nil {
			filtered := make([]Movie, 0)
			for _, movie := range movies {
				avgRating, err := GetAvarageRating(movie.Id)
				if err != nil {
					log.Printf("error with film: id=%d, err=%v", movie.Id, err)
					continue
				}
				if avgRating >= ratingThresholder {
					filtered = append(filtered, movie)
				}
			}
			movies = filtered
		}
	}

	topReviewed, err := GetTopReviewedFilms(5)
	if err != nil {
		log.Println("error getting top reviewed movies:", err)
		topReviewed = []Movie{}
	}
	newest, err := GetNewestMovies(5)
	if err != nil {
		log.Println("error getting newest movies:", err)
		newest = []Movie{}
	}

	movies = FillReviewCounts(movies)
	topReviewed = FillReviewCounts(topReviewed)
	newest = FillReviewCounts(newest)

	data := DiscoverPageData{
		Movies:      movies,
		TopReviewed: topReviewed,
		Newest:      newest,
		Page:        page,
		Genre:       selectedGenre,
		GenreID:     genreStr,
		Year:        yearStr,
		Rating:      ratingStr,
	}

	RenderDiscoverPage(w, data)
}

func FillReviewCounts(movies []Movie) []Movie {
	for i := range movies {
		count, err := GetReviewCount(movies[i].Id)
		if err != nil {
			log.Printf("Error getting review count for movie ID %d: %v", movies[i].Id, err)
			continue
		}
		movies[i].ReviewCount = count
	}
	return movies
}

func MovieHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	genreStr := r.URL.Query().Get("genre")
	yearStr := r.URL.Query().Get("year")
	ratingStr := r.URL.Query().Get("rating")
	log.Println("genreStr:", genreStr)
	log.Println("yearStr:", yearStr)
	log.Println("ratingStr:", ratingStr)
	if idStr != "" {
		parsed, err := strconv.Atoi(idStr)
		if err == nil {
			id := parsed
			movie, err := GetMovieById(id, genreStr, yearStr, ratingStr)
			if err == nil {
				RenderMoviePage(w, movie, genreStr, yearStr, ratingStr)
			} else {
				log.Println(err)
			}
		}
	} else {
		log.Printf("Movie with id %s does not exist\n", idStr)
	}
}

func HandleAddRating(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Println("ParseForm error:", err)
		http.Error(w, "error with form solving", http.StatusBadRequest)
		return
	}

	login := r.FormValue("login")
	movieIDStr := r.FormValue("movie_id")
	log.Println("AddRating: login =", login, ", movie_id =", movieIDStr)

	movieID, err := strconv.Atoi(movieIDStr)
	if err != nil {
		log.Println("Invalid movie_id:", movieIDStr, "error:", err)
		http.Error(w, "invalid movie_id", http.StatusBadRequest)
		return
	}

	ratingStr := r.FormValue("Rating")
	genreStr := r.FormValue("genre")
	yearStr := r.FormValue("year")
	selratingStr := r.FormValue("rating")

	log.Println("genreStr:", genreStr)
	log.Println("yearStr:", yearStr)
	log.Println("selratingStr:", selratingStr)

	log.Println("AddRating: rating =", ratingStr)
	rating, err := strconv.Atoi(ratingStr)
	if err != nil || rating < 1 || rating > 5 {
		log.Println("Invalid rating:", ratingStr, "error:", err)
		http.Error(w, "invalid rating", http.StatusBadRequest)
		return
	}

	userID, err := GetOrCreateUserIdByLogin(login)
	if err != nil {
		log.Println("GetOrCreateUserIdByLogin error:", err)
		http.Error(w, "Error with rating save", http.StatusInternalServerError)
		return
	}
	log.Println("UserID:", userID)

	err = SaveUserReviewOrRating(userID, movieID, rating, "")
	if err != nil {
		log.Println("SaveUserReviewOrRatingRating error:", err)
		http.Error(w, "Error with rating save", http.StatusInternalServerError)
		return
	}

	movie, err := GetMovieById(movieID, genreStr, yearStr, selratingStr)
	if err != nil {
		log.Println("error getting movie: ", err)
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	RenderMoviePage(w, movie, genreStr, yearStr, selratingStr)
}

func HandleAddReview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		log.Println("ParseForm error:", err)
		http.Error(w, "error with form solving", http.StatusBadRequest)
		return
	}

	login := r.FormValue("login")
	movieIDStr := r.FormValue("movie_id")
	comment := r.FormValue("Comment")
	genreStr := r.FormValue("genre")
	yearStr := r.FormValue("year")
	ratingStr := r.FormValue("rating")

	log.Println("AddReview: login =", login, ", movie_id =", movieIDStr, ", comment =", comment)

	movieID, err := strconv.Atoi(movieIDStr)
	if err != nil {
		log.Println("Invalid movie_id:", movieIDStr, "error:", err)
		http.Error(w, "invalid movie_id", http.StatusBadRequest)
		return
	}

	userID, err := GetOrCreateUserIdByLogin(login)
	if err != nil {
		log.Println("GetOrCreateUserIdByLogin error:", err)
		http.Error(w, "Error with review save", http.StatusInternalServerError)
		return
	}
	log.Println("UserID:", userID)

	err = SaveUserReviewOrRating(userID, movieID, 0, comment)
	if err != nil {
		log.Println("SaveUserReviewOrRatingRating error:", err)
		http.Error(w, "Error with review save", http.StatusInternalServerError)
		return
	}

	movie, err := GetMovieById(movieID, genreStr, yearStr, ratingStr)
	if err != nil {
		log.Println("error getting movie: ", err)
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	RenderMoviePage(w, movie, genreStr, yearStr, ratingStr)
}

func main() {
	err := SetWorkingDirToProjectRoot()
	if err != nil {
		log.Fatal("go.mod not found. It should exist in project root.")
	}

	err = InitTemplates()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", MainHandler)
	http.HandleFunc("/discover/", DiscoverHanderl)
	http.HandleFunc("/movies/", MovieHandler)
	http.HandleFunc("/add-rating", HandleAddRating)
	http.HandleFunc("/add-review", HandleAddReview)

	err = MakeDbConnection()
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	} else {
		log.Println("Database connection estableshed")
		defer CloseDbConnection()
	}

	fmt.Println("Server is running at http://localhost:8080/")
	err = http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func SetWorkingDirToProjectRoot() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	for {

		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {

			return os.Chdir(dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("go.mod not found in any parent directory")
		}
		dir = parent
	}
}
