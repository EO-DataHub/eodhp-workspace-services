// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/workspaces/{workspace-id}/{user-id}/s3-tokens": {
            "post": {
                "description": "Request S3 session credentials for user access to a single workspace. {user-id} can be set to \"me\" to use the token owner's user id.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "s3 credentials auth"
                ],
                "summary": "Request S3 session credentials",
                "parameters": [
                    {
                        "type": "string",
                        "example": "my-workspace",
                        "description": "Workspace ID",
                        "name": "workspace-id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "me",
                        "description": "User ID",
                        "name": "user-id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/handlers.S3Credentials"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "401": {
                        "description": "Unauthorized",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        },
        "/workspaces/{workspace-id}/{user-id}/sessions": {
            "post": {
                "description": "Request workspace scoped session credentials for user access to a single workspace. {user-id} can be set to \"me\" to use the token owner's user id.",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "workspace session credentials auth"
                ],
                "summary": "Request workspace scoped session credentials",
                "parameters": [
                    {
                        "type": "string",
                        "example": "my-workspace",
                        "description": "Workspace ID",
                        "name": "workspace-id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "string",
                        "example": "me",
                        "description": "User ID",
                        "name": "user-id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/handlers.AuthSessionResponse"
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "401": {
                        "description": "Unauthorized",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "403": {
                        "description": "Forbidden",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "string"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "handlers.AuthSessionResponse": {
            "type": "object",
            "properties": {
                "access": {
                    "type": "string"
                },
                "accessExpiry": {
                    "type": "string"
                },
                "refresh": {
                    "type": "string"
                },
                "refreshExpiry": {
                    "type": "string"
                },
                "scope": {
                    "type": "string"
                }
            }
        },
        "handlers.S3Credentials": {
            "type": "object",
            "properties": {
                "accessKeyId": {
                    "type": "string"
                },
                "expiration": {
                    "type": "string"
                },
                "secretAccessKey": {
                    "type": "string"
                },
                "sessionToken": {
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "v1",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "EODHP Workspace Services API",
	Description:      "This is the API for the EODHP Workspace Services.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
