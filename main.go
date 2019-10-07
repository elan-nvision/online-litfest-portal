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
	user     = "vish"
	password = "dontaskagain"
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
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./build/")))
	http.ListenAndServe(":8080", handlers.CORS(corsObj)(router))
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
