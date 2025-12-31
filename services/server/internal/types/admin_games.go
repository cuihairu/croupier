package types

type AdminGamesRequest struct {
	ID string `path:"id"`
}

type AdminGame struct {
	GameId   string   `json:"gameId"`
	GameName string   `json:"gameName,optional"`
	Envs     []string `json:"envs"`
}

type AdminGamesResponse struct {
	Games []AdminGame `json:"games"`
}

type AdminGamesUpdateRequest struct {
	ID    string      `path:"id"`
	Games []AdminGame `json:"games"`
}
