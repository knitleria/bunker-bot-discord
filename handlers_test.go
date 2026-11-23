package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type recordedResponse struct {
	method string
	path   string
	body   map[string]any
}

func fakeHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func commandInteraction(name string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID: "interaction", AppID: "application", Token: "test-token", GuildID: "guild",
		Type:   discordgo.InteractionApplicationCommand,
		Member: &discordgo.Member{User: &discordgo.User{ID: "user", Username: "Alice"}, Permissions: discordgo.PermissionAdministrator},
		Data:   discordgo.ApplicationCommandInteractionData{Name: name},
	}}
}

func setCommandOptions(i *discordgo.InteractionCreate, opts ...*discordgo.ApplicationCommandInteractionDataOption) {
	data := i.ApplicationCommandData()
	data.Options = opts
	i.Data = data
}

func setAttachment(i *discordgo.InteractionCreate, url string) {
	data := i.ApplicationCommandData()
	data.Options = []*discordgo.ApplicationCommandInteractionDataOption{{Name: "file", Type: discordgo.ApplicationCommandOptionAttachment, Value: "attachment"}}
	data.Resolved = &discordgo.ApplicationCommandInteractionDataResolved{Attachments: map[string]*discordgo.MessageAttachment{
		"attachment": {URL: url},
	}}
	i.Data = data
}

func newHandlerTest(t *testing.T) (*Bot, *discordgo.Session, *[]recordedResponse) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cf := exampleConfig(t)
	cf.Players[0].DiscordID = "user"
	if err := dealAll(cf.Players, cf.Pools, testRNG(1)); err != nil {
		t.Fatal(err)
	}
	store.games["guild"] = &Game{Players: cf.Players, Pools: cf.Pools}
	calls := new([]recordedResponse)
	session, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatal(err)
	}
	session.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		*calls = append(*calls, recordedResponse{req.Method, req.URL.Path, body})
		if req.Method == http.MethodPost {
			return fakeHTTPResponse(http.StatusNoContent, ""), nil
		}
		return fakeHTTPResponse(http.StatusOK, "{}"), nil
	})}
	return &Bot{store: store}, session, calls
}

func assertInteractionResponses(t *testing.T, calls []recordedResponse, deferred bool, content string) {
	t.Helper()
	wantCount, wantType := 1, discordgo.InteractionResponseChannelMessageWithSource
	if deferred {
		wantCount, wantType = 2, discordgo.InteractionResponseDeferredChannelMessageWithSource
	}
	if len(calls) != wantCount {
		t.Fatalf("got %d requests, want %d: %+v", len(calls), wantCount, calls)
	}
	initial := calls[0]
	if initial.method != http.MethodPost || !strings.HasSuffix(initial.path, "/callback") {
		t.Fatalf("not an initial response: %+v", initial)
	}
	if initial.body["type"] != float64(wantType) {
		t.Fatalf("response type: %v, want %v", initial.body["type"], wantType)
	}
	data, ok := initial.body["data"].(map[string]any)
	if !ok || data["flags"] != float64(discordgo.MessageFlagsEphemeral) {
		t.Fatalf("initial response is not ephemeral: %v", initial.body)
	}
	got, _ := data["content"].(string)
	if deferred {
		if got != "" {
			t.Fatalf("defer contains content %q", got)
		}
		edit := calls[1]
		if edit.method != http.MethodPatch || !strings.HasSuffix(edit.path, "/messages/@original") {
			t.Fatalf("not an original response edit: %+v", edit)
		}
		got, _ = edit.body["content"].(string)
	}
	if got == "" || !strings.Contains(got, content) {
		t.Fatalf("content %q does not contain %q", got, content)
	}
}

func TestCommandResponses(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		deferred      bool
	}{
		{"whoami", "Alice", false}, {"players", "Players:", true},
		{"mystats", "Твоя карточка", true}, {"preview", "Карточки игроков", true},
		{"generate", "generated successfully", true}, {"regenerate", "generated successfully", true},
		{"join", "joined the game", true}, {"unknown", "Unknown or unavailable", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			i := commandInteraction(tc.name)
			if tc.name == "join" {
				setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "nickname", Type: discordgo.ApplicationCommandOptionString, Value: "Иван"})
			}
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, tc.deferred, tc.content)
		})
	}
}

func TestGenerateResponsesOnErrors(t *testing.T) {
	for _, command := range []string{"generate", "regenerate", "join"} {
		for _, failure := range []string{"empty game", "save failure"} {
			t.Run(command+"/"+failure, func(t *testing.T) {
				bot, session, calls := newHandlerTest(t)
				i := commandInteraction(command)
				setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "nickname", Type: discordgo.ApplicationCommandOptionString, Value: "Иван"})
				if failure == "empty game" {
					bot.store.games["guild"] = &Game{}
				} else {
					bot.store.dir = filepath.Join(t.TempDir(), "missing")
				}
				bot.onInteraction(session, i)
				content := "Error:"
				if command == "join" && failure == "empty game" {
					content = "not a player"
				}
				assertInteractionResponses(t, *calls, true, content)
			})
		}
	}
}

func TestMyStatsWithoutCard(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(map[bool]string{false: "not joined", true: "not generated"}[linked], func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			bot.store.games["guild"] = &Game{}
			want := "not a player"
			if linked {
				bot.store.games["guild"].Players = []*Player{{Nickname: "Alice", DiscordID: "user"}}
				want = "not set your stats"
			}
			bot.onInteraction(session, commandInteraction("mystats"))
			assertInteractionResponses(t, *calls, true, want)
		})
	}
}

func TestCommandsRequireGuild(t *testing.T) {
	for _, command := range []string{"upload", "players", "mystats", "join", "generate", "preview", "regenerate", "setstat", "send", "start", "reset", "reroll"} {
		t.Run(command, func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			bot.store = nil // Any accidental store access will be caught by the response assertion.
			i := commandInteraction(command)
			i.GuildID, i.Member, i.User = "", nil, &discordgo.User{ID: "user"}
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, false, "only be used in a server")
		})
	}
	bot, session, calls := newHandlerTest(t)
	i := commandInteraction("whoami")
	i.GuildID, i.User, i.Member = "", i.Member.User, nil
	bot.onInteraction(session, i)
	assertInteractionResponses(t, *calls, false, "Alice")
}

func TestAdminCommandsDenyNonAdmin(t *testing.T) {
	for _, command := range []string{"upload", "players", "generate", "preview", "regenerate"} {
		t.Run(command, func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			bot.store = nil
			i := commandInteraction(command)
			i.Member.Permissions = 0
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, false, "not authorized")
		})
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		setup         func(*discordgo.InteractionCreate)
	}{
		{"missing file", "upload", func(i *discordgo.InteractionCreate) {}},
		{"nil option", "upload", func(i *discordgo.InteractionCreate) { setCommandOptions(i, nil) }},
		{"wrong attachment type", "upload", func(i *discordgo.InteractionCreate) {
			setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "file", Type: discordgo.ApplicationCommandOptionString, Value: "a"})
		}},
		{"missing resolved", "upload", func(i *discordgo.InteractionCreate) {
			setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "file", Type: discordgo.ApplicationCommandOptionAttachment, Value: "a"})
		}},
		{"nil attachment", "upload", func(i *discordgo.InteractionCreate) {
			setAttachment(i, "https://attachment.test/config")
			data := i.ApplicationCommandData()
			data.Resolved.Attachments["attachment"] = nil
		}},
		{"wrong value type", "upload", func(i *discordgo.InteractionCreate) {
			setAttachment(i, "https://attachment.test/config")
			i.ApplicationCommandData().Options[0].Value = 42
		}},
		{"empty attachment URL", "upload", func(i *discordgo.InteractionCreate) { setAttachment(i, "") }},
		{"missing nickname", "join", func(i *discordgo.InteractionCreate) {}},
		{"blank nickname", "join", func(i *discordgo.InteractionCreate) {
			setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "nickname", Type: discordgo.ApplicationCommandOptionString, Value: " "})
		}},
		{"wrong nickname value", "join", func(i *discordgo.InteractionCreate) {
			setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "nickname", Type: discordgo.ApplicationCommandOptionString, Value: 42})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			bot.store = nil
			i := commandInteraction(tc.command)
			tc.setup(i)
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, false, "")
			if strings.Contains((*calls)[0].body["data"].(map[string]any)["content"].(string), "internal error") {
				t.Fatal("invalid input caused panic")
			}
		})
	}
}

func TestUploadResponses(t *testing.T) {
	config, err := os.ReadFile("testdata/config.example.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, content string
		status        int
		body          string
		saveFailure   bool
	}{
		{"success", "successfully", 200, string(config), false},
		{"download failure", "Error downloading", 404, "", false},
		{"invalid JSON", "Error parsing config", 200, "{", false},
		{"save failure", "Error updating store", 200, string(config), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			if tc.saveFailure {
				bot.store.dir = filepath.Join(t.TempDir(), "missing")
			}
			previous := http.DefaultClient
			t.Cleanup(func() { http.DefaultClient = previous })
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if len(*calls) != 1 || (*calls)[0].body["type"] != float64(discordgo.InteractionResponseDeferredChannelMessageWithSource) {
					t.Fatal("download started before defer")
				}
				return fakeHTTPResponse(tc.status, tc.body), nil
			})}
			i := commandInteraction("upload")
			setAttachment(i, "https://attachment.test/config")
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, true, tc.content)
		})
	}
}

func TestRecoverRespondsBeforeAndAfterAcknowledgment(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "before", true: "after"}[deferred], func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			i := commandInteraction("preview")
			if deferred {
				bot.store = nil
			} else {
				i.Data = nil
			}
			var logs bytes.Buffer
			old := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(old) })
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, deferred, "internal error")
			if !strings.Contains(logs.String(), "Panic handling interaction") {
				t.Fatal("panic was not logged")
			}
		})
	}
}

func TestFailedDeferStopsWork(t *testing.T) {
	for _, command := range []string{"upload", "players", "mystats", "join", "generate", "preview", "regenerate"} {
		t.Run(command, func(t *testing.T) {
			bot, session, _ := newHandlerTest(t)
			game := bot.store.games["guild"]
			before, err := json.Marshal(game)
			if err != nil {
				t.Fatal(err)
			}
			i := commandInteraction(command)
			if command == "upload" {
				setAttachment(i, "https://attachment.test/config")
			}
			if command == "join" {
				setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "nickname", Type: discordgo.ApplicationCommandOptionString, Value: "Мария"})
			}
			count := 0
			session.Client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				count++
				return nil, errors.New("Discord unavailable")
			})
			previous := http.DefaultClient
			t.Cleanup(func() { http.DefaultClient = previous })
			http.DefaultClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatal("download after failed defer")
				return nil, errors.New("unexpected download")
			})}
			bot.onInteraction(session, i)
			after, err := json.Marshal(game)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("game changed after failed defer")
			}
			if _, err := os.Stat(filepath.Join(bot.store.dir, "guild.json")); !os.IsNotExist(err) {
				t.Fatal("store written after failed defer")
			}
			if count != 1 {
				t.Fatalf("got %d requests after failed defer", count)
			}
		})
	}
}

func TestResponseHelpersReturnAPIErrors(t *testing.T) {
	for _, op := range []string{"initial", "defer", "edit"} {
		t.Run(op, func(t *testing.T) {
			_, session, _ := newHandlerTest(t)
			session.Client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return fakeHTTPResponse(400, `{"message":"bad request","code":50035}`), nil
			})
			r := &interactionResponder{session: session, interaction: commandInteraction("whoami"), acknowledged: op == "edit"}
			var logs bytes.Buffer
			old := log.Writer()
			log.SetOutput(&logs)
			t.Cleanup(func() { log.SetOutput(old) })
			var err error
			switch op {
			case "initial":
				err = r.respond("test")
			case "defer":
				err = r.deferResponse()
			case "edit":
				err = r.editResponse("test")
			}
			if err == nil || !strings.Contains(logs.String(), "failed") {
				t.Fatal("API error was not returned and logged")
			}
			if op != "edit" && r.acknowledged {
				t.Fatal("failed response marked as acknowledged")
			}
		})
	}
}

func TestResponseHelpersKeepSingleInitialResponse(t *testing.T) {
	_, session, calls := newHandlerTest(t)
	r := &interactionResponder{session: session, interaction: commandInteraction("whoami")}
	if err := r.editResponse("too early"); err == nil {
		t.Fatal("edit before acknowledgment should fail")
	}
	if len(*calls) != 0 {
		t.Fatal("edit before acknowledgment called Discord")
	}
	if err := r.deferResponse(); err != nil {
		t.Fatal(err)
	}
	if err := r.deferResponse(); err == nil {
		t.Fatal("duplicate defer should fail")
	}
	if err := r.respond("completed"); err != nil {
		t.Fatal(err)
	}
	assertInteractionResponses(t, *calls, true, "completed")
}

func TestMissingUserRespondsWithoutPanic(t *testing.T) {
	for _, command := range []string{"whoami", "mystats", "join"} {
		t.Run(command, func(t *testing.T) {
			bot, session, calls := newHandlerTest(t)
			i := commandInteraction(command)
			i.Member.User = nil
			if command == "join" {
				setCommandOptions(i, &discordgo.ApplicationCommandInteractionDataOption{Name: "nickname", Type: discordgo.ApplicationCommandOptionString, Value: "Иван"})
			}
			bot.onInteraction(session, i)
			assertInteractionResponses(t, *calls, false, "Unable to identify")
		})
	}
}

func TestIgnoreNonCommandInteractions(t *testing.T) {
	bot, session, calls := newHandlerTest(t)
	bot.onInteraction(session, nil)
	bot.onInteraction(session, &discordgo.InteractionCreate{})
	i := commandInteraction("preview")
	i.Type = discordgo.InteractionMessageComponent
	bot.onInteraction(session, i)
	if len(*calls) != 0 {
		t.Fatal("non-command interaction caused a response")
	}
}
