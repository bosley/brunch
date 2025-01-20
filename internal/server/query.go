package server

import (
	"fmt"
	"net/http"

	"github.com/bosley/brunch/api"
)

func (s *Server) executeQuery(username string, op api.BrunchOp, key string, value string) (api.BrunchQueryResponse, error) {
	response := api.BrunchQueryResponse{
		Code:    http.StatusInternalServerError,
		Message: "FAILURE",
		Result:  "",
	}

	switch op {
	case api.BrunchOpRead:
		value, err := s.kvs.GetUserData(username, key)
		if err != nil {
			response.Message = fmt.Sprintf("Failed to read data: %v", err)
			return response, err
		}
		response.Code = http.StatusOK
		response.Message = "SUCCESS"
		response.Result = value

	case api.BrunchOpCreate:
		err := s.kvs.SetUserData(username, key, value)
		if err != nil {
			response.Message = fmt.Sprintf("Failed to create data: %v", err)
			return response, err
		}
		response.Code = http.StatusCreated
		response.Message = "SUCCESS"
		response.Result = value

	case api.BrunchOpUpdate:
		err := s.kvs.SetUserData(username, key, value)
		if err != nil {
			response.Message = fmt.Sprintf("Failed to update data: %v", err)
			return response, err
		}
		response.Code = http.StatusOK
		response.Message = "SUCCESS"
		response.Result = value

	case api.BrunchOpDelete:
		err := s.kvs.DeleteUserData(username, key)
		if err != nil {
			response.Message = fmt.Sprintf("Failed to delete data: %v", err)
			return response, err
		}
		response.Code = http.StatusOK
		response.Message = "SUCCESS"

	default:
		response.Code = http.StatusBadRequest
		response.Message = "Invalid operation"
		return response, fmt.Errorf("invalid operation: %s", op)
	}

	return response, nil
}
