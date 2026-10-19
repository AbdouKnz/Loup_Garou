package filestore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type example struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

func TestLoadFichierAbsentNestPasUneErreur(t *testing.T) {
	target := example{Name: "initial", Values: []string{"a"}}

	ok, err := Load(filepath.Join(t.TempDir(), "absent.json"), &target)
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if ok {
		t.Error("un fichier absent ne doit pas être signalé comme lu")
	}
	if target.Name != "initial" {
		t.Errorf("la cible a été modifiée : %+v", target)
	}
}

func TestSavePuisLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "donnees", "exemple.json")

	want := example{Name: "Loup-Garou", Values: []string{"Alice", "Bob"}}
	if err := Save(path, want); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	var got example
	ok, err := Load(path, &got)
	if err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}
	if !ok {
		t.Fatal("le fichier écrit devrait exister")
	}
	if got.Name != want.Name || len(got.Values) != len(want.Values) {
		t.Errorf("relu %+v, attendu %+v", got, want)
	}
}

func TestSaveNeLaisseAucunTemporaire(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exemple.json")

	if err := Save(path, example{Name: "un"}); err != nil {
		t.Fatalf("première écriture impossible : %v", err)
	}
	// Deuxième écriture : le fichier existe déjà, le remplacement doit passer.
	if err := Save(path, example{Name: "deux"}); err != nil {
		t.Fatalf("remplacement impossible : %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("dossier illisible : %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "exemple.json" {
		t.Errorf("contenu du dossier inattendu : %v", entries)
	}

	var got example
	if _, err := Load(path, &got); err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}
	if got.Name != "deux" {
		t.Errorf("name = %q, attendu %q", got.Name, "deux")
	}
}

func TestLoadFichierIllisible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "casse.json")
	if err := os.WriteFile(path, []byte("{pas du json"), 0o600); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	var target example
	if _, err := Load(path, &target); err == nil {
		t.Fatal("un fichier illisible doit être signalé")
	}
}

func TestWriteEcritDesOctetsBruts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note.txt")

	if err := Write(path, []byte("bonjour")); err != nil {
		t.Fatalf("écriture impossible : %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("lecture impossible : %v", err)
	}
	if strings.TrimSpace(string(raw)) != "bonjour" {
		t.Errorf("contenu = %q", raw)
	}
}
