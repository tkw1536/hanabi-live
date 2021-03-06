package main

import (
	"time"

	melody "gopkg.in/olahol/melody.v1"
)

// websocketConnect is fired when a new Melody WebSocket session is established
// This is the third step of logging in; users will only get here if authentication was successful
func websocketConnect(ms *melody.Session) {
	// Lock the command mutex for the duration of the function to ensure synchronous execution
	commandMutex.Lock()
	defer commandMutex.Unlock()

	// Turn the Melody session into a custom session
	s := &Session{ms}

	// Disconnect any existing connections with this username
	if s2, ok := sessions[s.UserID()]; ok {
		logger.Info("Closing existing connection for user \"" + s.Username() + "\".")
		s2.Error("You have logged on from somewhere else, so you have been disconnected here.")
		if err := s2.Close(); err != nil {
			logger.Info("Attempted to manually close a WebSocket connection, but it failed.")
		} else {
			logger.Info("Successfully terminated a WebSocket connection.")
		}

		// The connection is now closed, but the disconnect event will be fired in another goroutine
		// Thus, we need to manually call the function now to ensure that
		// the user is removed from existing games and so forth
		websocketDisconnect2(s2)
	}

	// Add the connection to a session map so that we can keep track of all of the connections
	sessions[s.UserID()] = s
	logger.Info("User \""+s.Username()+"\" connected;", len(sessions), "user(s) now connected.")

	// Get their total number of games played
	var totalGames int
	if v, err := models.Games.GetUserNumGames(s.UserID(), true); err != nil {
		logger.Error("Failed to get the number of games played for user \""+s.Username()+"\":", err)
		return
	} else {
		totalGames = v
	}

	// Get their settings from the database
	var settings Settings
	if v, err := models.UserSettings.Get(s.UserID()); err != nil {
		logger.Error("Failed to get the settings for user \""+s.Username()+"\":", err)
		return
	} else {
		settings = v
	}

	// They have successfully logged in, so send the initial message to the client
	type HelloMessage struct {
		ID            int      `json:"id"`
		Username      string   `json:"username"`
		TotalGames    int      `json:"totalGames"`
		Admin         bool     `json:"admin"`
		Muted         bool     `json:"muted"`
		FirstTimeUser bool     `json:"firstTimeUser"`
		Settings      Settings `json:"settings"`
		ShuttingDown  bool     `json:"shuttingDown"`
	}
	s.Emit("hello", &HelloMessage{
		// Send the user their corresponding user ID
		ID: s.UserID(),

		// We have to send the username back to the client because they may
		// have logged in with the wrong case, and the client needs to know
		// their exact username or various bugs will occur
		Username: s.Username(),

		// We also send the total amount of games that they have played
		// (to be shown in the nav bar on the history page)
		TotalGames: totalGames,

		Admin:         s.Admin(),         // Some users can perform admin-only commands
		Muted:         s.Muted(),         // Some users are muted (as a resulting of spamming, etc.)
		FirstTimeUser: s.FirstTimeUser(), // First time users get a quick tutorial

		// The various client settings are stored server-side so that users can seamlessly
		// transition between computers
		Settings: settings,

		// Also let the user know if the server is currently performing a graceful shutdown
		ShuttingDown: shuttingDown,
	})

	// Send them a random name
	commandGetName(s, nil)

	// Alert everyone that a new user has logged in
	// (note that we intentionally send users a message about themselves)
	notifyAllUser(s)

	// Send a "userList" message
	// (this is much more performant than sending an individual "user" message for every user)
	userMessageList := make([]*UserMessage, 0)
	for _, s2 := range sessions {
		// Skip sending a message about ourselves since we already sent that above
		if s2.UserID() != s.UserID() {
			userMessageList = append(userMessageList, makeUserMessage(s2))
		}
	}
	s.Emit("userList", userMessageList)

	// Send a "tableList" message
	// (this is much more performant than sending an individual "table" message for every table)
	tableMessageList := make([]*TableMessage, 0)
	for _, t := range tables {
		if t.Visible {
			tableMessageList = append(tableMessageList, makeTableMessage(s, t))
		}
	}
	s.Emit("tableList", tableMessageList)

	// Send the past 50 chat messages from the lobby
	chatSendPastFromDatabase(s, "lobby", 50)

	// Send them the message(s) of the day
	msg := "Find teammates and discuss strategy in the "
	msg += "<a href=\"https://discord.gg/FADvkJp\" target=\"_blank\" rel=\"noopener noreferrer\">"
	msg += "Hanabi Discord chat</a>."
	s.Emit("chat", &ChatMessage{
		Msg:      msg,
		Server:   true,
		Datetime: time.Now(),
		Room:     "lobby",
	})

	// Send the user's game history
	// (only the last 10 games to prevent wasted bandwidth)
	var history []*GameHistory
	if v, err := models.Games.GetUserHistory(s.UserID(), 0, 10, false); err != nil {
		logger.Error("Failed to get the history for user \""+s.Username()+"\":", err)
		return
	} else {
		history = v
	}
	history = historyFillVariants(history)
	s.NotifyGameHistory(history, false)

	// First, check to see if this user was in any existing games
	for _, t := range tables {
		if t.Replay {
			continue
		}

		for _, p := range t.Players {
			if p.Name != s.Username() {
				continue
			}

			// Update the player object with the new socket
			p.Session = s

			// Add the player back to the game
			logger.Info(t.GetName() + "Automatically reattending player \"" + s.Username() + "\".")
			commandTableReattend(s, &CommandData{
				TableID: t.ID,
			})
			// (this function does not care what their current game and/or status is)

			// If the user happens to be in both a game and a replay, then ignore the replay
			return
		}
	}

	// Second, check to see if this user was in any existing shared replays
	for _, t := range tables {
		if !t.Replay {
			continue
		}

		for id := range t.DisconSpectators {
			if id == s.UserID() {
				delete(t.DisconSpectators, s.UserID())

				// Add the player back to the shared replay
				logger.Info(t.GetName() + "Automatically re-spectating player " +
					"\"" + s.Username() + "\".")
				// This function does not care what their current game and/or status is
				commandTableSpectate(s, &CommandData{
					TableID: t.ID,
				})

				// We can return here because the player can only be in one shared replay at a time
				return
			}
		}
	}
}
