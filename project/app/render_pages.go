// ------------------
// | Page rendering |
// -----------------

package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

func repeat(s string, count int) string {
	return strings.Repeat(s, count)
}

func seq(start, end int) []int {
	s := make([]int, end-start+1)
	for i := range s {
		s[i] = start + 1
	}
	return s
}

func add(a, b int) int {
	return a + b
}

var templates struct {
	Main     *template.Template
	Discover *template.Template
	Movie    *template.Template
}

func InitTemplates() error {
	tmplDir, err := GetTemplatesDir()
	if err != nil {
		return err
	}

	funcMap := template.FuncMap{
		"seq":    seq,
		"repeat": repeat,
		"add":    add,
	}

	templates.Main = template.Must(template.New("base.html").
		Funcs(funcMap).
		ParseFiles(
			path.Join(tmplDir, "base.html"),
			path.Join(tmplDir, "main.html"),
		))

	templates.Discover = template.Must(template.New("base.html").
		Funcs(funcMap).
		ParseFiles(
			path.Join(tmplDir, "base.html"),
			path.Join(tmplDir, "discover.html"),
		))

	templates.Movie = template.Must(template.New("base.html").
		Funcs(funcMap).
		ParseFiles(
			path.Join(tmplDir, "base.html"),
			path.Join(tmplDir, "movie.html"),
		))

	return nil
}

func RenderMainPage(w http.ResponseWriter) {
	err := templates.Main.Execute(w, nil)
	if err != nil {
		log.Fatal("Error when executing template: ", err)
	}
}

func RenderDiscoverPage(w http.ResponseWriter, data DiscoverPageData) {
	err := templates.Discover.Execute(w, data)
	if err != nil {
		log.Fatal("Error when executing template: ", err)
	}
}

func RenderMoviePage(w http.ResponseWriter, movie Movie, genreStr string, yearStr string, ratingStr string) {
	reviews, err := GetReviewsByMovieID(movie.Id)
	if err != nil {
		log.Println("Error getting reviews: ", err)
	}

	avgRating, err := GetAvarageRating(movie.Id)
	if err != nil {
		log.Println("Error getting average rating:", err)
	}

	ratingDist, err := GetRatingDistribution(movie.Id)
	if err != nil {
		log.Println("Error getting rating distribution:", err)
	}

	data := struct {
		Movie        Movie
		Reviews      []ReviewWithUser
		Avarage      float64
		RatingCounts map[int]int
		GenreId      string
		Year         string
		Rating       string
		FiveStars    []int
	}{
		Movie:        movie,
		Reviews:      reviews,
		Avarage:      avgRating,
		RatingCounts: ratingDist,
		GenreId:      genreStr,
		Year:         yearStr,
		Rating:       ratingStr,
		FiveStars:    []int{1, 2, 3, 4, 5},
	}

	err = templates.Movie.Execute(w, data)
	if err != nil {
		log.Fatal("Error when executing template: ", err)
	}
}

func GetTemplatesDir() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory %w", err)
	}
	return path.Join(root, "web", "templates"), nil
}
