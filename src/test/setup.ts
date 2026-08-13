import '@testing-library/jest-dom';
import { beforeEach } from 'vitest';
import { mockCurrentUserSession } from '../data/mockData';

// Pre-seed mock session in test environment
beforeEach(() => {
  localStorage.setItem('pcp_auth_session', JSON.stringify(mockCurrentUserSession));
});
