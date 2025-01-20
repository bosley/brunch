package server

import (
	"fmt"
	"net/http"

	"github.com/bosley/brunch/api"
)

func (s *Server) executeQuery(query string) (api.BrunchQueryResponse, error) {
	response := api.BrunchQueryResponse{
		Code:    http.StatusInternalServerError,
		Message: "FAILURE",
		Result:  "",
	}
	fmt.Println("query:", query)
	return response, nil
}

/*
The brunch server hosts a bunch of trees, so its a forrest database


we will use a simple key/server for now that maps the conversation names to their serialized data.


*/
