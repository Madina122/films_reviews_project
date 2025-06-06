package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

var db *sql.DB

func MakeDbConnection() error {
	var err error
	connStr := os.Getenv("DATABASE_URL")
	connStr := "user=postgres password=YOUR_PASSWORD dbname=project_go sslmode=disable"
	dbTmp, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	if err = dbTmp.Ping(); err != nil {
		return err
	}

	db = dbTmp // <-- очень важно!
	return nil
}

func CloseDbConnection() {
	db.Close()
}

func GetMovies() ([]Movie, error) {
	query := `SELECT id, title, poster_url, description, release_date FROM movies`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying database: %w", err)
	}
	defer rows.Close()

	movies := make([]Movie, 0)
	for rows.Next() {
		m := Movie{}
		err := rows.Scan(&m.Id, &m.Name, &m.PosterUrl, &m.Description, &m.ReleaseDate)
		if err != nil {
			return movies, fmt.Errorf("error scanning row: %w", err)
		}
		movies = append(movies, m)
	}

	if err = rows.Err(); err != nil {
		return movies, fmt.Errorf("error iterating rows: %w", err)
	}

	return movies, nil
}

func GetMovieById(id int, genreStr, yearStr, ratingStr string) (Movie, error) {
	query := "SELECT id, title, release_date, genre_ids, poster_url, description FROM movies WHERE id = $1"
	row := db.QueryRow(query, id)

	m := Movie{}
	var genreIdsStr string
	err := row.Scan(&m.Id, &m.Name, &m.ReleaseDate, &genreIdsStr, &m.PosterUrl, &m.Description)
	if err != nil {
		if err == sql.ErrNoRows {
			return m, fmt.Errorf("Movie with id %d not found", id)
		}
		return m, fmt.Errorf("error scanning row: %w", err)
	}

	m.Genres, err = parseGenreIds(genreIdsStr)
	if err != nil {
		return m, fmt.Errorf("error parsing genre ids: %w", err)
	}
	m.SelectedGenre = genreStr
	m.SelectedYear = yearStr
	m.SelectedRating = ratingStr

	return m, nil
}

func parseGenreIds(genreIdsStr string) ([]int, error) {
	if genreIdsStr == "" {
		return []int{}, nil
	}

	parts := strings.Split(genreIdsStr, ",")
	genreIds := make([]int, len(parts))

	for i, idStr := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(idStr))
		if err != nil {
			return genreIds, fmt.Errorf("invalid genre ID '%s': %w", idStr, err)
		}
		genreIds[i] = id
	}
	return genreIds, nil
}

func GetMoviesByGenre(genreId int) ([]Movie, error) {
	query := `SELECT id, title, poster_url, description, release_date FROM movies WHERE genre_ids LIKE $1`
	likePattern := fmt.Sprintf("%%%d%%", genreId) // genre_id is stored as comma-separated string
	rows, err := db.Query(query, likePattern)
	if err != nil {
		return nil, fmt.Errorf("error querying database: %w", err)
	}
	defer rows.Close()

	movies := make([]Movie, 0)
	for rows.Next() {
		m := Movie{}
		err := rows.Scan(&m.Id, &m.Name, &m.PosterUrl, &m.Description, &m.ReleaseDate)
		if err != nil {
			return movies, fmt.Errorf("error scanning row: %w", err)
		}
		movies = append(movies, m)
	}

	if err = rows.Err(); err != nil {
		return movies, fmt.Errorf("error iterating rows: %w", err)
	}

	return movies, nil
}

func GetMoviesByYear(year int) ([]Movie, error) {
	// Используем EXTRACT для получения года из даты
	query := `SELECT id, title, poster_url, description, release_date 
              FROM movies 
              WHERE EXTRACT(YEAR FROM release_date) = $1`

	rows, err := db.Query(query, year)
	if err != nil {
		return nil, fmt.Errorf("error querying database: %w", err)
	}
	defer rows.Close()

	movies := make([]Movie, 0)
	for rows.Next() {
		m := Movie{}
		// Добавляем release_date в Scan
		err := rows.Scan(&m.Id, &m.Name, &m.PosterUrl, &m.Description, &m.ReleaseDate)
		if err != nil {
			return movies, fmt.Errorf("error scanning row: %w", err)
		}
		movies = append(movies, m)
	}

	if err = rows.Err(); err != nil {
		return movies, fmt.Errorf("error iterating rows: %w", err)
	}

	return movies, nil
}

func GetMoviesByGenreAndYear(genreId, year int) ([]Movie, error) {
	query := `SELECT id, title, poster_url, description, release_date 
              FROM movies 
              WHERE genre_ids LIKE $1 AND EXTRACT(YEAR FROM release_date) = $2`

	likePattern := fmt.Sprintf("%%%d%%", genreId)
	rows, err := db.Query(query, likePattern, year)
	if err != nil {
		return nil, fmt.Errorf("error querying database: %w", err)
	}
	defer rows.Close()

	movies := make([]Movie, 0)
	for rows.Next() {
		m := Movie{}
		// Добавляем release_date в Scan
		err := rows.Scan(&m.Id, &m.Name, &m.PosterUrl, &m.Description, &m.ReleaseDate)
		if err != nil {
			return movies, fmt.Errorf("error scanning row: %w", err)
		}
		movies = append(movies, m)
	}

	if err = rows.Err(); err != nil {
		return movies, fmt.Errorf("error iterating rows: %w", err)
	}

	return movies, nil
}

// Get users who wrote a review
func GetUserByMoviesReviews(MovieId int) ([]User, error) {
	query := `
		SELECT DISTINCT u.id, u.login, u.email, u.phone
		FROM users u
		JOIN reviews r ON u.id = r.user_id
		WHERE r.movie_id = $1
	`
	rows, err := db.Query(query, MovieId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(&u.Id, &u.Login, &u.Email, &u.Phone)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func GetReviewsByMovieID(MovieId int) ([]ReviewWithUser, error) {
	query := `
		SELECT r.rating, r.comment, r.created_at, u.login
		FROM reviews r 
		JOIN users u ON r.user_id = u.id 
		WHERE r.movie_id = $1 and r.comment <> ''
		ORDER BY r.created_at DESC
	`
	rows, err := db.Query(query, MovieId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []ReviewWithUser
	for rows.Next() {
		var r ReviewWithUser
		err := rows.Scan(&r.Rating, &r.Comment, &r.CreatedAt, &r.UserLogin)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, nil
}

func GetAvarageRating(MovieId int) (float64, error) {
	query := `SELECT AVG(rating) FROM reviews WHERE movie_id = $1 and rating between 1 and 5`
	var avg sql.NullFloat64
	err := db.QueryRow(query, MovieId).Scan(&avg)
	if err != nil {
		return 0, err
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

func GetRatingDistribution(movieID int) (map[int]int, error) {
	query := `
		SELECT rating, COUNT(*) 
		FROM reviews 
		WHERE movie_id = $1 and rating != 0
		GROUP BY rating
	`
	rows, err := db.Query(query, movieID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dist := make(map[int]int)
	for rows.Next() {
		var rating, count int
		if err := rows.Scan(&rating, &count); err != nil {
			return nil, err
		}
		dist[rating] = count
	}
	return dist, nil
}

func GetOrCreateUserIdByLogin(login string) (int, error) {
	query := `
		Select Id from users where login = $1
	`
	var UserId int
	err := db.QueryRow(query, login).Scan(&UserId)
	if err == sql.ErrNoRows {
		err = db.QueryRow(`
			insert into users (login) Values ($1) returning id
		`, login).Scan(&UserId)
	}
	return UserId, err
}

func SaveUserReviewOrRating(userID, movieID int, rating int, comment string) error {
	var exitstingId int
	var existingMovieId int
	var existingRating sql.NullInt64
	var existingComment sql.NullString

	err := db.QueryRow(`
		select id, movie_id, rating, comment
		from reviews
		where movie_id = $1 and user_id = $2 
		order by created_at desc
		limit 1;
	`, movieID, userID).Scan(&exitstingId, &existingMovieId, &existingRating, &existingComment)

	if err == sql.ErrNoRows {
		_, err = db.Exec(`
			insert into reviews (movie_id, user_id, rating, comment)
			values($1, $2, $3, $4)
		`, movieID, userID, rating, comment)
		return err
	}

	if err != nil {
		return err
	}
	print(existingMovieId)
	print(movieID)

	if existingMovieId != movieID {
		_, err = db.Exec(`
			insert into reviews (movie_id, user_id, rating, comment) values($1, $2, $3, $4)
		`, movieID, userID, rating, comment)
		return err
	} else {
		if rating != 0 {
			_, err = db.Exec(`
				update reviews
				set rating = $1
				where id = $2
			`, rating, exitstingId)
			if err != nil {
				return err
			}
		}
	}

	if comment != "" {
		if !existingComment.Valid || existingComment.String == "" {
			_, err = db.Exec(`
				update reviews
				set comment = $1, created_at = current_timestamp
				where id = $2
			`, comment, exitstingId)
			return err
		} else {
			_, err = db.Exec(`
				insert into reviews (movie_id, user_id, rating, comment)
				values($1, $2, 0, $3)
			`, movieID, userID, comment)
			return err
		}
	}
	return nil
}

func GetTopReviewedFilms(limit int) ([]Movie, error) {
	query := `
		SELECT m.id, m.title, m.poster_url
		FROM movies m
		JOIN reviews r ON m.id = r.movie_id
		GROUP BY m.id
		ORDER BY COUNT(r.id) DESC
		LIMIT $1;
	`
	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		var m Movie
		err := rows.Scan(&m.Id, &m.Name, &m.PosterUrl)
		if err != nil {
			return nil, err
		}
		movies = append(movies, m)
	}
	return movies, nil
}
func GetReviewCount(movieId int) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM reviews WHERE movie_id = $1 and comment <> '' `
	err := db.QueryRow(query, movieId).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func GetNewestMovies(limit int) ([]Movie, error) {
	rows, err := db.Query(`
		SELECT id, title, poster_url, release_date, description
		FROM movies
		ORDER BY release_date DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var movies []Movie
	for rows.Next() {
		var m Movie
		err := rows.Scan(&m.Id, &m.Name, &m.PosterUrl, &m.ReleaseDate, &m.Description)
		if err != nil {
			log.Println("Error scanning movie:", err)
			continue
		}
		movies = append(movies, m)
	}
	return movies, nil
}
