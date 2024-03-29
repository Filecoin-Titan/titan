package validation

import (
	"math"
	"math/rand"
	"time"
)

var (
	firstElectionInterval = 5 * time.Minute    // Time of the first election
	electionCycle         = 1 * 24 * time.Hour // Election cycle
)

func getTimeAfter(t time.Duration) time.Time {
	return time.Now().Add(t)
}

// triggers the election process at a regular interval.
func (m *Manager) startElectionTicker() {
	validators, err := m.nodeMgr.LoadValidators(m.nodeMgr.ServerID)
	if err != nil {
		log.Errorf("electionTicker LoadValidators err: %v", err)
		return
	}

	expiration := m.getElectionCycle()
	if len(validators) <= 0 {
		expiration = firstElectionInterval
	}

	m.nextElectionTime = getTimeAfter(expiration)

	ticker := time.NewTicker(expiration)
	defer ticker.Stop()

	doElect := func() {
		m.nextElectionTime = getTimeAfter(m.getElectionCycle())

		ticker.Reset(m.getElectionCycle())
		err := m.elect()
		if err != nil {
			log.Errorf("elect err:%s", err.Error())
		}
	}

	for {
		select {
		case <-ticker.C:
			doElect()
		case <-m.updateCh:
			doElect()
		}
	}
}

// elect triggers an election and updates the list of validators.
func (m *Manager) elect() error {
	log.Debugln("start elect ")
	validators, validatables := m.electValidators()

	m.ResetValidatorGroup(validators, validatables)

	return m.nodeMgr.UpdateValidators(validators, m.nodeMgr.ServerID)
}

func (m *Manager) CompulsoryElection(validators []string) error {
	list, _ := m.nodeMgr.GetAllValidCandidateNodes()

	validatables := make([]string, 0)
	for _, nid := range list {
		isV := false

		for _, nid2 := range validators {
			if nid == nid2 {
				isV = true
			}
		}

		if !isV {
			validatables = append(validatables, nid)
		}
	}

	m.ResetValidatorGroup(validators, validatables)

	return m.nodeMgr.UpdateValidators(validators, m.nodeMgr.ServerID)
}

// StartElection triggers an election manually.
func (m *Manager) StartElection() {
	// TODO need to add restrictions to disallow frequent calls?
	m.updateCh <- struct{}{}
}

// returns the ratio of validators that should be elected, based on the scheduler configuration.
func (m *Manager) getValidatorRatio() float64 {
	cfg, err := m.config()
	if err != nil {
		log.Errorf("get schedulerConfig err:%s", err.Error())
		return 0
	}

	return cfg.ValidatorRatio
}

// returns the election cycle
func (m *Manager) getElectionCycle() time.Duration {
	cfg, err := m.config()
	if err != nil {
		log.Errorf("get schedulerConfig err:%s", err.Error())
		return electionCycle
	}

	return time.Duration(cfg.ElectionCycle*24) * time.Hour
}

// performs the election process and returns the list of elected validators.
func (m *Manager) electValidators() ([]string, []string) {
	ratio := m.getValidatorRatio()

	list, _ := m.nodeMgr.GetAllValidCandidateNodes()

	needValidatorCount := int(math.Ceil(float64(len(list)) * ratio))
	if needValidatorCount <= 0 {
		return nil, list
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(list), func(i, j int) {
		list[i], list[j] = list[j], list[i]
	})

	if needValidatorCount > len(list) {
		needValidatorCount = len(list)
	}

	validators := list[:needValidatorCount]
	validatables := list[needValidatorCount:]

	return validators, validatables
}
