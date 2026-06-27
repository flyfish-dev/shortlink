package web

import "embed"

//go:embed static/* templates/*
var FS embed.FS
