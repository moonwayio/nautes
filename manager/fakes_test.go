// Copyright 2025 The Moonway.io Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package manager

// fakeComponent implements Component for testing.
type fakeComponent struct {
	name  string
	start func() error
	stop  func() error
}

func (m *fakeComponent) Start() error {
	if m.start != nil {
		return m.start()
	}
	return nil
}

func (m *fakeComponent) Stop() error {
	if m.stop != nil {
		return m.stop()
	}
	return nil
}

func (m *fakeComponent) GetName() string {
	return m.name
}

// fakeLeaderElectionAwareComponent implements LeaderElectionAware for testing.
type fakeLeaderElectionAwareComponent struct {
	name                string
	start               func() error
	stop                func() error
	needsLeaderElection bool
}

func (m *fakeLeaderElectionAwareComponent) Start() error {
	if m.start != nil {
		return m.start()
	}
	return nil
}

func (m *fakeLeaderElectionAwareComponent) Stop() error {
	if m.stop != nil {
		return m.stop()
	}
	return nil
}

func (m *fakeLeaderElectionAwareComponent) GetName() string {
	return m.name
}

func (m *fakeLeaderElectionAwareComponent) NeedsLeaderElection() bool {
	return m.needsLeaderElection
}

// fakeLeaderElectionSubscriber implements ElectionSubscriber for testing.
type fakeLeaderElectionSubscriber struct {
	name     string
	callback func()
	start    func() error
	stop     func() error
}

func (m *fakeLeaderElectionSubscriber) Start() error {
	if m.start != nil {
		return m.start()
	}
	return nil
}

func (m *fakeLeaderElectionSubscriber) Stop() error {
	if m.stop != nil {
		return m.stop()
	}
	return nil
}

func (m *fakeLeaderElectionSubscriber) OnStartLeading() {
	if m.callback != nil {
		m.callback()
	}
}

func (m *fakeLeaderElectionSubscriber) OnStopLeading() {
	if m.callback != nil {
		m.callback()
	}
}

func (m *fakeLeaderElectionSubscriber) GetName() string {
	return m.name
}
