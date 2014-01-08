package peergrouper

import (
	"fmt"
	"net"
	"sort"

	"launchpad.net/juju-core/replicaset"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/instance"
)

type peerGroupInfo struct {
	machines  []*machine
	statuses  []replicaset.Status
	members   []replicaset.Member
	mongoPort int
}

type machine struct {
	id        string
	candidate bool
	addresses []instance.Address

	// Set by desiredPeerGroup
	voting bool
}

// getPeerGroupInfo collates current session information about the
// mongo peer group with information from state machines.
func getPeerGroupInfo(st *state.State, ms []*state.Machine) (*peerGroupInfo, error) {
	session := st.MongoSession()
	info := &peerGroupInfo{}
	var err error
	info.statuses, err = replicaset.CurrentStatus(session)
	if err != nil {
		return nil, fmt.Errorf("cannot get replica set status: %v", err)
	}
	info.members, err = replicaset.CurrentMembers(session)
	if err != nil {
		return nil, fmt.Errorf("cannot get replica set members: %v", err)
	}
	for _, m := range ms {
		info.machines = append(info.machines, &machine{
			id:        m.Id(),
			candidate: m.IsCandidate(),
			addresses: m.Addresses(),
		})
	}
	return info, nil
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

// desiredPeerGroup returns the mongo peer group according
// to the given servers. It may return (nil, nil) if the current
// group is already correct.
func desiredPeerGroup(info *peerGroupInfo) ([]replicaset.Member, error) {
	changed := false
	members, extra := info.membersMap()
	// We may find extra peer group members if the machines
	// have been removed or their state server status removed.
	// This should only happen if they had been set to non-voting
	// before removal, in which case we want to remove it
	// from the members list. If we find a member that's still configured
	// to vote, it's an error.
	// TODO There are some other possibilities
	// for what to do in that case.
	// 1) leave them untouched, but deal
	// with others as usual "i didn't see that bit"
	// 2) leave them untouched, deal with others,
	// but make sure the extras aren't eligible to
	// be primary.
	// 3) remove them "get rid of bad rubbish"
	// 4) bomb out "run in circles, scream and shout"
	// 5) do nothing "nothing to see here"
	for _, member := range extra {
		if member.Votes != nil && *member.Votes > 0 {
			return nil, fmt.Errorf("voting non-machine member found in peer group")
		}
		changed = true
	}
	statuses := info.statusesMap(members)

	var toRemoveVote, toAddVote, toKeep []*machine
	for _, m := range info.machines {
		member := members[m]
		isVoting := member != nil && member.Votes != nil && *member.Votes > 0
		m.voting = isVoting
		switch {
		case m.candidate && isVoting:
			toKeep = append(toKeep, m)
			// No need to do anything.
		case m.candidate && !isVoting:
			if status, ok := statuses[m]; ok && isReady(status) {
				toAddVote = append(toAddVote, m)
			}
		case !m.candidate && isVoting:
			toRemoveVote = append(toRemoveVote, m)
		case !m.candidate && !isVoting:
			toKeep = append(toKeep, m)
		}
	}
	// sort machines to be added and removed so that we
	// get deterministic behaviour when testing. Earlier
	// entries will be dealt with preferentially, so we could
	// potentially sort by some other metric in each case.
	sort.Sort(byId(toRemoveVote))
	sort.Sort(byId(toAddVote))

	setVoting := func(m *machine, voting bool) {
		setMemberVoting(members[m], voting)
		m.voting = voting
		changed = true
	}

	// Remove voting members if they can be replaced by
	// candidates that are ready. This does not affect
	// the total number of votes.
	nreplace := min(len(toRemoveVote), len(toAddVote))
	for i := 0; i < nreplace; i++ {
		from := toRemoveVote[i]
		to := toAddVote[i]
		setVoting(from, false)
		setVoting(to, true)
	}
	toAddVote = toAddVote[nreplace:]
	toRemoveVote = toRemoveVote[nreplace:]

	// At this point, one or both of toAdd or toRemove
	// is empty, so we can adjust the voting-member count
	// by an even delta, maintaining the invariant
	// that the total vote count is odd.
	if len(toAddVote) > 0 {
		toAddVote = toAddVote[0 : len(toAddVote)-len(toAddVote)%2]
		for _, m := range toAddVote {
			setVoting(m, true)
		}
	} else {
		toRemoveVote = toRemoveVote[0 : len(toRemoveVote)-len(toRemoveVote)%2]
		for _, m := range toRemoveVote {
			setVoting(m, false)
		}
	}
	for _, m := range toKeep {
		if members[m] == nil {
			// This machine was not previously in the members list,
			// so add it (as non-voting).
			member := &replicaset.Member{
				Tags: map[string]string{
					"juju-machine-id": m.id,
				},
			}
			members[m] = member
			setVoting(m, false)
		}
	}
	// Make sure all members' machine addresses are up to date.
	for _, m := range info.machines {
		addr := instance.SelectInternalAddress(m.addresses)
		if addr == "" {
			continue
		}
		// TODO ensure that replicset works correctly with IPv6 [host]:port addresses.
		addr = net.JoinHostPort(addr, fmt.Sprint(info.mongoPort))
		if addr != members[m].Address {
			members[m].Address = addr
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}
	var memberSet []replicaset.Member
	for _, member := range members {
		memberSet = append(memberSet, *member)
	}
	return memberSet, nil
}

func setMemberVoting(member *replicaset.Member, voting bool) {
	if voting {
		member.Votes = nil
		member.Priority = nil
	} else {
		votes := 0
		member.Votes = &votes
		priority := 0.0
		member.Priority = &priority
	}
}

type byId []*machine

func (l byId) Len() int           { return len(l) }
func (l byId) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l byId) Less(i, j int) bool { return l[i].id < l[j].id }

func (info *peerGroupInfo) membersMap() (members map[*machine]*replicaset.Member, extra []replicaset.Member) {
	members = make(map[*machine]*replicaset.Member)
	for i := range info.members {
		member := &info.members[i]
		var found *machine
		if mid, ok := member.Tags["juju-machine-id"]; ok {
			for _, m := range info.machines {
				if m.id == mid {
					found = m
				}
			}
		}
		if found != nil {
			members[found] = member
		} else {
			extra = append(extra, *member)
		}
	}
	return members, extra
}

func (info *peerGroupInfo) statusesMap(members map[*machine] *replicaset.Member) map[*machine]replicaset.Status {
	statuses := make(map[*machine]replicaset.Status)
	for _, status := range info.statuses {
		for m, member := range members {
			if member.Id == status.Id {
				statuses[m] = status
				break
			}
		}
	}
	return statuses
}
