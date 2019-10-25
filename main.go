package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"regexp"
	"time"

	jwtmiddleware "github.com/auth0/go-jwt-middleware"
	"github.com/codegangsta/negroni"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"gopkg.in/gomail.v2"
)

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))
var rxEmail = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

type ParticipantDatabaseObject struct {
	Fname     string
	Lname     string
	Institute string
	Email     string
	Age       int
	Rcode     string
	Events    []int
}

type MemeifyObject struct {
	Email   string `json:"email"`
	Entries int    `json:"entries"`
}

type Response struct {
	Message string `json:"message"`
}

type Jwks struct {
	Keys []JSONWebKeys `json:"keys"`
}

type JSONWebKeys struct {
	Kty string   `json:"kty"`
	Kid string   `json:"kid"`
	Use string   `json:"use"`
	N   string   `json:"n"`
	E   string   `json:"e"`
	X5c []string `json:"x5c"`
}

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "your-password"
	dbname   = "online_litfest"
)

var db *sql.DB

func main() {
	corsObj := handlers.AllowedOrigins([]string{"*"})
	var err error
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(50000)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(time.Nanosecond)
	fmt.Println("Set up postgre db")
	jwtMiddleware := jwtmiddleware.New(jwtmiddleware.Options{
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			// Verify 'aud' claim
			aud := "https://online-litfest.auth0.com/api/v2/"
			checkAud := token.Claims.(jwt.MapClaims).VerifyAudience(aud, false)
			if !checkAud {
				return token, errors.New("Invalid audience.")
			}
			// Verify 'iss' claim
			iss := "https://online-litfest.auth0.com/"
			checkIss := token.Claims.(jwt.MapClaims).VerifyIssuer(iss, false)
			if !checkIss {
				return token, errors.New("Invalid issuer.")
			}

			cert, err := getPemCert(token)
			if err != nil {
				panic(err.Error())
			}

			result, _ := jwt.ParseRSAPublicKeyFromPEM([]byte(cert))
			return result, nil
		},
		SigningMethod: jwt.SigningMethodRS256,
	})
	router := mux.NewRouter()
	router.Handle("/api/private/memeify/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-images", email+".*.png")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM memeify_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE memeify_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO memeify_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Memeify")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/memeify/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM memeify_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM memeify_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/sweetheart/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-sweetheart", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM sweetheart_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE sweetheart_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO sweetheart_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/sweetheart/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM sweetheart_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM sweetheart_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/sweetheart/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-sweetheart", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM sweetheart_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE sweetheart_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO sweetheart_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/essay/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-essay", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM essay_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE essay_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO essay_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/essay/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM essay_entries WHERE email = $1)`, email)
			if err != nil {

				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM essay_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/essay/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-essay", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM essay_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE essay_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO essay_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/goosebumps/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-goosebumps", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM goosebumps_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE goosebumps_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO goosebumps_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/goosebumps/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM goosebumps_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM goosebumps_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/goosebumps/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-goosebumps", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM goosebumps_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE goosebumps SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO goosebumps (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/dearme/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-dearme", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM dearme_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE dearme_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO dearme_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/dearme/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM dearme_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM dearme_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/dearme/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-dearme", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM dearme_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE dearme_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO dearme_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/ragtag/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-ragtag", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM ragtag_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE ragtag_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO ragtag_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/ragtag/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM ragtag_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM ragtag_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/ragtag/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-ragtag", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM ragtag_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE ragtag_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO ragtag_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/review/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-review", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM review_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE review_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO review_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/review/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM review_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM review_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/review/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-review", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM review_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE review_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO review_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/plot/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-plot", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM plot_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE plot_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO plot_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/plot/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM plot_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM plot_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/plot/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-plot", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM plot_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE plot_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO plot_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.Handle("/api/private/poetry/upload", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-poetry", email+".*.html")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Event")
			m.SetBody("text/html", string(fileBytes)+"<br />This is an automatically generated submission. ")

			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")

			// Send the email to Bob, Cora and Dan.
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM poetry_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE poetry_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO poetry_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
		}))))
	router.Handle("/api/private/poetry/entries", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			email := claims["email"].(string)
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM poetry_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			var user MemeifyObject
			if doesEntryExist {
				rows, err = db.Query(`SELECT * FROM poetry_entries WHERE email = $1`, email)
				if err != nil {
					fmt.Println("Error in reading DB object", err)
				}
				rows.Next()
				err = rows.Scan(&user.Email, &user.Entries)
				if err != nil {
					fmt.Println("Error in parsing", err)
				}
			} else {
				user.Email = "noonecares"
				user.Entries = 0
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			json.NewEncoder(w).Encode(user)
		}))))
	router.Handle("/api/private/poetry/pdf", negroni.New(
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := r.URL.Query()["id_token"][0]
			claims := jwt.MapClaims{}
			jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				x, err := getPemCert(token)
				return []byte(x), err
			})
			r.ParseMultipartForm(10 << 20)
			file, handler, err := r.FormFile("file")
			if err != nil {
				fmt.Println("Error Retrieving the File")
				fmt.Println(err)
				return
			}
			defer file.Close()
			fmt.Printf("Uploaded File: %+v\n", handler.Filename)
			fmt.Printf("File Size: %+v\n", handler.Size)
			fmt.Printf("MIME Header: %+v\n", handler.Header)
			email := claims["email"].(string)
			tempFile, err := ioutil.TempFile("temp-poetry", email+".*.pdf")
			if err != nil {
				fmt.Println(err)
			}
			defer tempFile.Close()
			fileBytes, err := ioutil.ReadAll(file)
			if err != nil {
				fmt.Println(err)
			}
			// write this byte array to our temporary file
			tempFile.Write(fileBytes)
			// return that we have successfully uploaded our file!
			fmt.Fprintf(w, "Successfully Uploaded File\n")
			rows, err := db.Query(`SELECT EXISTS(SELECT * FROM poetry_entries WHERE email = $1)`, email)
			if err != nil {
				fmt.Println("Error in reading DB object", err)
			}
			var doesEntryExist bool
			rows.Next()
			err = rows.Scan(&doesEntryExist)
			if err != nil {
				fmt.Println("Error in parsing", err)
			}
			if doesEntryExist {
				rows, err = db.Query(`UPDATE poetry_entries SET entries = entries + 1 WHERE email = $1`, email)
			} else {
				sqlStatement := `INSERT INTO poetry_entries (email, entries) VALUES ($1, $2)`
				_, err = db.Exec(sqlStatement, email, 1)
				if err != nil {
					panic(err)
				}
			}
			m := gomail.NewMessage()
			m.SetHeader("From", "web@elan.org.in")
			m.SetHeader("To", "ma18btech11011@iith.ac.in")
			m.SetHeader("Subject", "New submission for Sweetheart")
			m.SetBody("text/html", "The submission has been attached. <br />This is an automatically generated submission. ")
			m.Attach(tempFile.Name())
			d := gomail.NewDialer("smtp.gmail.com", 587, "web@elan.org.in", "web@2020")
			if err := d.DialAndSend(m); err != nil {
				panic(err)
			}
		}))))
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./build/")))

	http.ListenAndServe(":8000", handlers.CORS(corsObj)(router))
}
func getPemCert(token *jwt.Token) (string, error) {
	cert := ""
	resp, err := http.Get("https://online-litfest.auth0.com/.well-known/jwks.json")

	if err != nil {

		return cert, err
	}
	defer resp.Body.Close()

	var jwks = Jwks{}
	err = json.NewDecoder(resp.Body).Decode(&jwks)
	if err != nil {
		return cert, err
	}

	for k, _ := range jwks.Keys {
		if token.Header["kid"] == jwks.Keys[k].Kid {
			cert = "-----BEGIN CERTIFICATE-----\n" + jwks.Keys[k].X5c[0] + "\n-----END CERTIFICATE-----"
		}
	}

	if cert == "" {
		err := errors.New("Unable to find appropriate key.")
		return cert, err
	}

	return cert, nil
}
func responseJSON(message string, w http.ResponseWriter, statusCode int) {
	response := Response{message}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(jsonResponse)
}
