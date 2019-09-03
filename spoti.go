package main

// TODO get devices
// TODO play tracks
// TODO reorder tracks in a playlist

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

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
			Name:  "search",
			Usage: "Search for tracks.",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:     "query",
					Required: true,
					Usage:    "The search query.",
				},
			},
			Action: func(c *cli.Context) error {
				query := c.String("query")
				client := getClient()

				// TODO Allow playlist, album and artist search.
				results, err := client.Search(query, spotify.SearchTypeTrack)
				if err != nil {
					log.Fatal(err)
				}

				for _, track := range results.Tracks.Tracks {
					fmt.Println()
					fmt.Println("Name:", track.Name)

					artistNames := make([]string, len(track.Artists))
					for index, artist := range track.Artists {
						artistNames[index] = artist.Name
					}

					fmt.Println("Artists:", strings.Join(artistNames, ", "))
					fmt.Println("Album:", track.Album.Name)
					fmt.Println("ID:", track.ID)
					fmt.Println("URI:", track.URI)
				}

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
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:     "playlistID, p",
							Required: true,
							Usage:    "The playlist ID.",
						},
					},
					Action: func(c *cli.Context) error {
						playlistID := spotify.ID(c.String("playlistID"))
						client := getClient()

						playlist, err := client.GetPlaylist(playlistID)
						if err != nil {
							log.Fatal(err)
						}

						printPlaylist(playlist)

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
				{
					Name:  "create",
					Usage: "Create a playlist",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:     "name, n",
							Required: true,
							Usage:    "The name of the playlist.",
						},
					},
					Action: func(c *cli.Context) error {
						name := c.String("name")
						if len(name) < 1 {
							log.Fatal("Name is required.")
						}

						client := getClient()
						user := getUser()

						playlist, err := client.CreatePlaylistForUser(user.ID, name, "", true)
						if err != nil {
							log.Fatal(err)
						}

						printPlaylist(playlist)

						return nil
					},
				},
				{
					Name:  "add-tracks",
					Usage: "Add tracks to a playlist",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:     "playlistID, p",
							Required: true,
							Usage:    "The playlist ID.",
						},
						cli.StringSliceFlag{
							Name:     "trackID, t",
							Required: true,
							Usage:    "The IDs of the tracks to add to the playlist",
						},
					},
					Action: func(c *cli.Context) error {
						playlistID := spotify.ID(c.String("playlistID"))
						client := getClient()
						trackIDs := getTrackIds(c)

						snapshot, err := client.AddTracksToPlaylist(playlistID, trackIDs...)
						if err != nil {
							log.Fatal(err)
						}

						fmt.Println(snapshot)

						return nil
					},
				},
				{
					Name:  "remove-tracks",
					Usage: "Remove tracks from a playlist.",
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:     "playlistID, p",
							Required: true,
							Usage:    "The playlist ID.",
						},
						cli.StringSliceFlag{
							Name:     "trackID, t",
							Required: true,
							Usage:    "The IDs of the tracks to remove from the playlist.",
						},
					},
					Action: func(c *cli.Context) error {
						playlistID := spotify.ID(c.String("playlistID"))
						trackIDs := getTrackIds(c)
						client := getClient()

						snapshot, err := client.RemoveTracksFromPlaylist(playlistID, trackIDs...)
						if err != nil {
							log.Fatal(err)
						}

						fmt.Println(snapshot)

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

func getTrackIds(c *cli.Context) []spotify.ID {
	trackIDs := c.StringSlice(("trackID"))
	trackIDsConverted := make([]spotify.ID, len(trackIDs))
	for index, id := range trackIDs {
		trackIDsConverted[index] = spotify.ID(id)
	}
	return trackIDsConverted
}

func printPlaylist(playlist *spotify.FullPlaylist) {
	fmt.Println("Name:", playlist.Name)
	fmt.Println("ID:", playlist.ID)
	fmt.Println("Owner:", playlist.Owner.DisplayName)
	fmt.Println("Public:", playlist.IsPublic)
	fmt.Println("URI:", playlist.URI)
	fmt.Println("Description:", playlist.Description)
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
