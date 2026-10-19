// Package filestore lit et écrit de petits fichiers JSON, sans dépendance.
//
// C'est le seul endroit du serveur qui touche le disque : les comptes et les
// sessions y sont confiés. Une écriture remplace toujours le fichier d'un bloc
// (temporaire puis renommage) : un arrêt en cours d'écriture ne peut pas
// laisser des données tronquées à la place des vraies.
package filestore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Load lit un fichier JSON dans dst.
//
// Un fichier absent n'est pas une erreur : dst reste inchangé et le booléen dit
// s'il y avait quelque chose à lire. C'est le cas normal au premier démarrage.
func Load(path string, dst any) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, fmt.Errorf("lecture de %s : %w", path, err)
	}

	if err := json.Unmarshal(raw, dst); err != nil {
		return false, fmt.Errorf("fichier %s illisible : %w", path, err)
	}

	return true, nil
}

// Save écrit v dans path, en JSON lisible, de façon atomique.
func Save(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encodage de %s : %w", path, err)
	}

	return Write(path, append(raw, '\n'))
}

// Write écrit des octets de façon atomique : dossier créé au besoin, fichier
// temporaire dans le même dossier, synchronisation, puis renommage.
func Write(path string, raw []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("dossier %s : %w", dir, err)
	}

	file, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("fichier temporaire pour %s : %w", path, err)
	}

	tmp := file.Name()

	// Un retour en erreur ne doit laisser aucun temporaire derrière lui.
	defer func() { _ = os.Remove(tmp) }()

	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return fmt.Errorf("écriture de %s : %w", path, err)
	}

	// Les données doivent être sur le disque avant le renommage, sans quoi un
	// arrêt brutal pourrait laisser un fichier renommé mais vide.
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("synchronisation de %s : %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("fermeture de %s : %w", path, err)
	}

	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("renommage vers %s : %w", path, err)
	}

	return nil
}
