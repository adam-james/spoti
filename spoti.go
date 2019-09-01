package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli"
	"github.com/zmb3/spotify"
	"golang.org/x/oauth2"
)

const (
	redirectURI = "http://localhost:3000/callback"
	userFile    = "./tmp/user.json"
	tokenFile   = "./tmp/token.json"
)

var (
	auth  = spotify.NewAuthenticator(redirectURI, spotify.ScopeUserReadPrivate)
	ch    = make(chan *spotify.Client)
	state = "abc123"
)

func main() {
	app := cli.NewApp()
	app.Name = "spoti"
	app.Usage = "A CLI for the Spotify Web API."

	app.Commands = []cli.Command{
		{
			Name:  "login",
			Usage: "Log in to Spotify.",
			Action: func(c *cli.Context) error {
				startServer()
				redirectToLogin()

				client := <-ch

				user, err := client.CurrentUser()
				if err != nil {
					log.Fatal(err)
				}

				json, err := json.MarshalIndent(user, "", "    ")
				if err != nil {
					log.Fatal(err)
				}

				err = ioutil.WriteFile("./tmp/user.json", json, 0600)
				if err != nil {
					log.Fatal(err)
				}

				fmt.Println("Logged in as:", user.ID)

				return nil
			},
		},
		{
			Name:  "me",
			Usage: "Get current user.",
			Action: func(c *cli.Context) error {
				user, err := getUser()
				if err != nil {
					log.Fatal(err)
				}

				json, err := json.MarshalIndent(user, "", "    ")
				if err != nil {
					log.Fatal(err)
				}

				fmt.Println(string(json))
				return nil
			},
		},
		{
			Name:  "playlist",
			Usage: "Manage playlists",
			Subcommands: []cli.Command{
				{
					Name:  "list",
					Usage: "List your playlists.",
					Action: func(c *cli.Context) error {
						client, err := getClient()
						if err != nil {
							log.Fatal(err)
						}

						user, err := getUser()
						if err != nil {
							log.Fatal(err)
						}

						playlists, err := client.GetPlaylistsForUser(user.ID)
						if err != nil {
							log.Fatal(err)
						}

						json, err := json.MarshalIndent(playlists, "", "    ")
						if err != nil {
							log.Fatal(err)
						}

						fmt.Println(string(json))

						return nil
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func startServer() {
	http.HandleFunc("/callback", completeAuth)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Got request for:", r.URL.String())
	})
	go http.ListenAndServe(":3000", nil)
}

func completeAuth(w http.ResponseWriter, r *http.Request) {
	tok, err := auth.Token(state, r)
	if err != nil {
		http.Error(w, "Couldn't get token", http.StatusForbidden)
		log.Fatal(err)
	}
	if st := r.FormValue("state"); st != state {
		http.NotFound(w, r)
		log.Fatalf("State mismatch: %s != %s\n", st, state)
	}

	json, err := json.MarshalIndent(tok, "", "    ")
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile("./tmp/token.json", json, 0600)
	if err != nil {
		log.Fatal(err)
	}

	client := auth.NewClient(tok)
	fmt.Fprintf(w, "Login Completed!")
	ch <- &client
}

func redirectToLogin() {
	url := auth.AuthURL(state)
	// TODO this only works on macOS
	err := exec.Command("open", url).Start()
	if err != nil {
		log.Fatal(err)
	}
}

func getUser() (spotify.User, error) {
	var user spotify.User
	data, err := ioutil.ReadFile(userFile)
	err = json.Unmarshal(data, &user)
	return user, err
}

func getToken() (oauth2.Token, error) {
	var token oauth2.Token
	data, err := ioutil.ReadFile(tokenFile)
	err = json.Unmarshal(data, &token)
	return token, err
}

func getClient() (spotify.Client, error) {
	tok, err := getToken()
	client := auth.NewClient(&tok)
	return client, err
}
