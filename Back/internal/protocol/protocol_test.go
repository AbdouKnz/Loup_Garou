package protocol

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// checkCode vérifie le code d'erreur attendu ; un code vide signifie « aucune erreur ».
func checkCode(t *testing.T, err error, wantErr ErrorCode) {
	t.Helper()

	if wantErr == "" {
		if err != nil {
			t.Fatalf("erreur inattendue : %v", err)
		}
		return
	}

	if got := CodeOf(err); got != wantErr {
		t.Fatalf("code d'erreur = %q, attendu %q (erreur : %v)", got, wantErr, err)
	}
}

func TestNormalizeRoomCode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr ErrorCode
	}{
		{name: "minuscules acceptées", raw: "abc234", want: "ABC234"},
		{name: "espaces ignorés", raw: "  ABC234  ", want: "ABC234"},
		{name: "trop court", raw: "ABC23", wantErr: CodeInvalidCode},
		{name: "trop long", raw: "ABC2345", wantErr: CodeInvalidCode},
		{name: "caractère ambigu refusé", raw: "ABC10O", wantErr: CodeInvalidCode},
		{name: "vide", raw: "", wantErr: CodeInvalidCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeRoomCode(tt.raw)
			checkCode(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("code = %q, attendu %q", got, tt.want)
			}
		})
	}
}

func TestNormalizePseudo(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr ErrorCode
	}{
		{name: "simple", raw: "Alice", want: "Alice"},
		{name: "espaces réduits", raw: "  Jean   Loup  ", want: "Jean Loup"},
		{name: "accents conservés", raw: "Amélie", want: "Amélie"},
		{name: "trop court", raw: "A", wantErr: CodeInvalidPseudo},
		{name: "trop long", raw: strings.Repeat("a", MaxPseudoLength+1), wantErr: CodeInvalidPseudo},
		{name: "longueur limite acceptée", raw: strings.Repeat("a", MaxPseudoLength), want: strings.Repeat("a", MaxPseudoLength)},
		{name: "espaces seuls", raw: "   ", wantErr: CodeInvalidPseudo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePseudo(tt.raw)
			checkCode(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("pseudo = %q, attendu %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeChat(t *testing.T) {
	long := strings.Repeat("a", MaxChatLength+1)

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr ErrorCode
	}{
		{name: "message normal", raw: "  Bonjour  ", want: "Bonjour"},
		{name: "vide refusé", raw: "   ", wantErr: CodeInvalidMessage},
		{name: "trop long refusé", raw: long, wantErr: CodeInvalidMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeChat(tt.raw)
			checkCode(t, err, tt.wantErr)
			if got != tt.want {
				t.Errorf("message = %q, attendu %q", got, tt.want)
			}
		})
	}
}

func TestEnvelopeDecodeData(t *testing.T) {
	env := MustEncode(TypeChatSend, ChatSendPayload{Text: "coucou"})

	var payload ChatSendPayload
	if err := env.DecodeData(&payload); err != nil {
		t.Fatalf("décodage impossible : %v", err)
	}
	if payload.Text != "coucou" {
		t.Errorf("texte = %q, attendu %q", payload.Text, "coucou")
	}

	if err := (Envelope{Type: TypeChatSend}).DecodeData(&payload); CodeOf(err) != CodeInvalidMessage {
		t.Errorf("charge utile manquante : code = %q", CodeOf(err))
	}

	if err := (Envelope{Type: TypeChatSend, Data: json.RawMessage(`[1,2]`)}).DecodeData(&payload); CodeOf(err) != CodeInvalidMessage {
		t.Errorf("charge utile illisible : code = %q", CodeOf(err))
	}
}

func TestMustEncodeTrameSansDonnees(t *testing.T) {
	env := MustEncode(TypePong, nil)
	if env.Data != nil {
		t.Errorf("data = %s, attendu vide", env.Data)
	}

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("encodage impossible : %v", err)
	}
	if string(raw) != `{"type":"pong"}` {
		t.Errorf("trame = %s", raw)
	}
}

func TestIsClientType(t *testing.T) {
	for _, accepted := range []MessageType{
		TypeAuth, TypeChatSend, TypePlayerReady, TypeGameStart, TypeGameAgain,
		TypeGameVote, TypeGameNightVote, TypeGameNightAction, TypeGameSpeakDone,
		TypeBotAdd, TypeBotRemove, TypePing,
	} {
		if !IsClientType(accepted) {
			t.Errorf("%q devrait être acceptée", accepted)
		}
	}

	for _, refused := range []MessageType{TypeRoomState, TypeChatMessage, TypeError, "inconnue"} {
		if IsClientType(refused) {
			t.Errorf("%q devrait être refusée", refused)
		}
	}
}

func TestErrorForEtStatus(t *testing.T) {
	err := NewError(CodeRoomFull, "La partie est complète (%d joueurs).", MaxPlayers)

	if got := CodeOf(err); got != CodeRoomFull {
		t.Errorf("code = %q, attendu %q", got, CodeRoomFull)
	}
	if !strings.Contains(err.Error(), "complète") {
		t.Errorf("message inattendu : %s", err)
	}

	tests := []struct {
		code ErrorCode
		want int
	}{
		{CodeInvalidMessage, http.StatusBadRequest},
		{CodeInvalidCode, http.StatusBadRequest},
		{CodeNotAuthenticated, http.StatusUnauthorized},
		{CodeUnauthorized, http.StatusForbidden},
		{CodeRoomNotFound, http.StatusNotFound},
		{CodeRoomFull, http.StatusConflict},
		{CodeRateLimited, http.StatusTooManyRequests},
		{CodeNotImplemented, http.StatusNotImplemented},
	}

	for _, tt := range tests {
		if got := StatusFor(NewError(tt.code, "peu importe")); got != tt.want {
			t.Errorf("StatusFor(%q) = %d, attendu %d", tt.code, got, tt.want)
		}
	}
}

func TestErreurTechniqueMasquee(t *testing.T) {
	technical := errors.New("connexion refusée : 127.0.0.1:5432")

	if got := CodeOf(technical); got != CodeInternal {
		t.Errorf("code = %q, attendu %q", got, CodeInternal)
	}
	if got := MessageOf(technical); strings.Contains(got, "127.0.0.1") {
		t.Errorf("le message technique a fuité : %q", got)
	}
	if got := StatusFor(technical); got != http.StatusInternalServerError {
		t.Errorf("statut = %d, attendu %d", got, http.StatusInternalServerError)
	}
}
