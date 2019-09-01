package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli"
	"github.com/zmb3/spotify"
)

const redirectURI = "http://localhost:3000/callback"

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
			Name:  "me",
			Usage: "Get current user.",
			Action: func(c *cli.Context) error {
				startServer()
				redirectToLogin()

				// wait for auth to complete
				client := <-ch

				// use the client to make calls that require authorization
				user, err := client.CurrentUser()
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
						startServer()
						redirectToLogin()

						// wait for auth to complete
						client := <-ch

						// use the client to make calls that require authorization
						user, err := client.CurrentUser()
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
	// use the token to get an authenticated client
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
