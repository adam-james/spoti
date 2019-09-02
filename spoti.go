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
	userFile    = "./user.json"
	tokenFile   = "./token.json"
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

				err = ioutil.WriteFile(userFile, json, 0600)
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
				user := getUser()
				fmt.Println("Display Name:", user.DisplayName)
				fmt.Println("ID:", user.ID)
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
						client := getClient()
						user := getUser()

						playlists, err := client.GetPlaylistsForUser(user.ID)
						if err != nil {
							log.Fatal(err)
						}

						for _, playlist := range playlists.Playlists {
							fmt.Println()
							fmt.Println("Name:", playlist.Name)
							fmt.Println("ID:", playlist.ID)
							fmt.Println("URI:", playlist.URI)
						}

						return nil
					},
				},
				{
					Name:  "details",
					Usage: "Show details for a single playlists",
					Action: func(c *cli.Context) error {
						if len(os.Args) < 4 {
							log.Fatal("You must provide an ID.")
						}

						id := spotify.ID(os.Args[3])
						client := getClient()

						playlist, err := client.GetPlaylist(id)
						if err != nil {
							log.Fatal(err)
						}

						fmt.Println("Name:", playlist.Name)
						fmt.Println("ID:", playlist.ID)
						fmt.Println("Owner:", playlist.Owner.DisplayName)
						fmt.Println("Public:", playlist.IsPublic)
						fmt.Println("URI:", playlist.URI)
						fmt.Println("Description:", playlist.Description)

						fmt.Println("Tracks:")
						for _, track := range playlist.Tracks.Tracks {
							fmt.Println("  - Name:", track.Track.Name)
							fmt.Println("    ID:", track.Track.ID)
							fmt.Println("    Added At:", track.AddedAt)
							fmt.Println("    Added By:", track.AddedBy.DisplayName)
						}

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

	err = ioutil.WriteFile(tokenFile, json, 0600)
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

func getUser() spotify.User {
	var user spotify.User

	data, err := ioutil.ReadFile(userFile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(data, &user)
	if err != nil {
		log.Fatal(err)
	}

	return user
}

func getToken() *oauth2.Token {
	var token oauth2.Token

	data, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		log.Fatal(err)
	}

	err = json.Unmarshal(data, &token)
	if err != nil {
		log.Fatal(err)
	}

	return &token
}

func getClient() spotify.Client {
	client := auth.NewClient(getToken())
	return client
}

// TODO create playlist
// TODO edit playlist
