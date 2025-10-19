package manager

func (m *Manager) flush() error {
	var err error
	if m.objectCache != nil {
		if e := m.objectCache.Flush(); err == nil && e != nil {
			err = e
		}
	}
	return err
}
