package multiplayer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type savedWorld struct {
	Schema                              int
	World                               string
	Epoch, EntityCounter, GroundCounter uint64
	Environment                         *Environment
	Entities                            []WorldActor
	Enemies                             []EnemyHealth
	Items                               []GroundItem
}

func (s *Server) openWorld() error {
	if !s.cfg.DedicatedWorld || s.cfg.StatePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.StatePath), 0700); err != nil {
		return err
	}
	file, err := lockWorldFile(s.cfg.StatePath + ".lock")
	if err != nil {
		return fmt.Errorf("mundo já aberto ou arquivo indisponível: %w", err)
	}
	s.worldFile = file
	if err = s.loadWorld(); err != nil {
		file.Close()
		s.worldFile = nil
		return err
	}
	return s.checkpointWorldLocked(true)
}
func (s *Server) loadWorld() error {
	f, err := os.Open(s.cfg.StatePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > 64<<20 {
		return fmt.Errorf("arquivo do mundo excede 64 MiB")
	}
	var saved savedWorld
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err = dec.Decode(&saved); err != nil {
		return fmt.Errorf("mundo inválido: %w", err)
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("dados após o mundo")
	}
	raw, err := hex.DecodeString(saved.World)
	if err != nil || len(raw) != 12 || saved.Schema != 1 || saved.Epoch == 0 || saved.Epoch >= math.MaxInt64 || saved.EntityCounter > math.MaxInt64 || saved.GroundCounter > MaxWorldItems ||
		len(saved.Entities) > maxWorldEntities || len(saved.Enemies) > maxSessionDamageActors || len(saved.Items) > MaxWorldItems {
		return fmt.Errorf("versão ou limites do mundo inválidos")
	}
	ids := map[uint64]bool{}
	for _, a := range saved.Entities {
		if !a.valid() || ids[a.ID] || a.ID > saved.EntityCounter || s.entities[a.key()] != nil {
			return fmt.Errorf("entidade persistida inválida")
		}
		ids[a.ID] = true
		s.entities[a.key()] = &worldEntity{State: a}
	}
	for _, e := range saved.Enemies {
		if !e.Valid() {
			return fmt.Errorf("vida persistida inválida")
		}
		if _, ok := s.enemies[e.key()]; ok {
			return fmt.Errorf("vida persistida duplicada")
		}
		s.enemies[e.key()] = e
	}
	for _, item := range saved.Items {
		if !item.Valid() || item.ID > saved.GroundCounter {
			return fmt.Errorf("item persistido inválido")
		}
		key := item.Source.key(item.Owner)
		if _, ok := s.groundItems[item.ID]; ok {
			return fmt.Errorf("item persistido duplicado")
		}
		if _, ok := s.groundKeys[key]; ok {
			return fmt.Errorf("origem persistida duplicada")
		}
		// A crash can occur after a native inventory write and before receipt.
		// Keep the reservation closed until its outcome can be reconciled.
		if item.State == "reserved" {
			item.State = "uncertain"
		}
		if item.State == "spawning" {
			item.State = "uncertain"
			item.Claimant = item.Owner
			item.Request = item.Source.Local
		}
		if err := s.pinLoot(item.Source); err != nil {
			return err
		}
		s.groundItems[item.ID] = item
		s.groundKeys[key] = item.ID
	}
	if saved.Environment != nil {
		if !saved.Environment.Valid() {
			return fmt.Errorf("ambiente persistido inválido")
		}
		s.environment = saved.Environment
	}
	s.groundWorld = saved.World
	s.environmentEpoch = saved.Epoch + 1
	s.entityCounter = saved.EntityCounter
	s.groundCounter = saved.GroundCounter
	return nil
}
func (s *Server) checkpointWorldLocked(force bool) error {
	if s.persistenceError != nil {
		return s.persistenceError
	}
	if s.worldFile == nil || !force && time.Since(s.savedAt) < 2*time.Second {
		return nil
	}
	value := savedWorld{Schema: 1, World: s.groundWorld, Epoch: s.environmentEpoch,
		EntityCounter: s.entityCounter, GroundCounter: s.groundCounter, Environment: s.environment}
	for _, e := range s.entities {
		value.Entities = append(value.Entities, e.State)
	}
	for _, e := range s.enemies {
		value.Enemies = append(value.Enemies, e)
	}
	for _, e := range s.groundItems {
		value.Items = append(value.Items, e)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return s.failWorldLocked(err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.cfg.StatePath), ".world-*.tmp")
	if err != nil {
		return s.failWorldLocked(err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, s.cfg.StatePath)
	}
	if err == nil && runtime.GOOS != "windows" {
		dir, openErr := os.Open(filepath.Dir(s.cfg.StatePath))
		if openErr != nil {
			err = openErr
		} else {
			err = dir.Sync()
			dir.Close()
		}
	}
	if err != nil {
		return s.failWorldLocked(err)
	}
	s.savedAt = time.Now()
	return nil
}
func (s *Server) failWorldLocked(err error) error {
	s.persistenceError = fmt.Errorf("gravação do mundo falhou, sessão encerrada: %w", err)
	select {
	case s.failures <- s.persistenceError:
	default:
	}
	s.cancel()
	s.listener.Close()
	for _, p := range s.peers {
		p.stop()
	}
	return s.persistenceError
}
func (s *Server) Errors() <-chan error { return s.failures }
