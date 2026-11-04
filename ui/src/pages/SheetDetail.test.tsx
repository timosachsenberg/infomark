// Smoke test: SheetDetail must render a student's task list with the submission
// affordance, driven entirely by a mocked API layer (no network).
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';

const { get, getAll, getPoints, getResult, courseGet, getOwnTeam } = vi.hoisted(() => ({
  get: vi.fn(),
  getAll: vi.fn(),
  getPoints: vi.fn(),
  getResult: vi.fn(),
  courseGet: vi.fn(),
  getOwnTeam: vi.fn(),
}));

vi.mock('@/api/agent', () => ({
  default: {
    Account: { getEnrollments: () => Promise.resolve([{ id: 0, course_id: 1, role: 0 }]) },
    Sheets: {
      get,
      getPoints,
    },
    Tasks: {
      getAll,
      getResult,
      submitTask: vi.fn(),
      downloadOwnSubmission: vi.fn(),
    },
    Courses: { get: courseGet },
    Teams: { getOwn: getOwnTeam },
  },
}));

import '@/i18n';
import SheetDetail from '@/pages/SheetDetail';

beforeEach(() => {
  vi.clearAllMocks();
  // Published in the past, due in the future -> submission enabled.
  get.mockResolvedValue({
    id: 1,
    name: 'Sheet 0',
    publish_at: '2020-01-01T00:00:00Z',
    due_at: '2999-01-01T00:00:00Z',
  });
  getAll.mockResolvedValue([{ id: 1, name: 'task 0', max_points: 13 }]);
  getPoints.mockResolvedValue([{ acquired_points: 5, achievable_points: 13, max_points: 13, task_id: 1 }]);
  getResult.mockRejectedValue({ status: 404 });
  courseGet.mockResolvedValue({ id: 1, name: 'Info 2', max_team_size: 1 });
  getOwnTeam.mockRejectedValue({ status: 404 });
});

const renderSheet = () =>
  render(
    <MemoryRouter initialEntries={['/courses/1/sheets/1']}>
      <Routes>
        <Route path="/courses/:courseId/sheets/:sheetId" element={<SheetDetail />} />
      </Routes>
    </MemoryRouter>,
  );

describe('SheetDetail (student)', () => {
  it('renders the task list and an enabled submit button', async () => {
    render(
      <MemoryRouter initialEntries={['/courses/1/sheets/1']}>
        <Routes>
          <Route path="/courses/:courseId/sheets/:sheetId" element={<SheetDetail />} />
        </Routes>
      </MemoryRouter>,
    );

    expect(await screen.findByText('task 0')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /submit/i })).toBeInTheDocument();
    });
    expect(screen.getByRole('button', { name: /submit/i })).not.toBeDisabled();
  });

  it('says that a team member hands in for the whole team', async () => {
    courseGet.mockResolvedValue({ id: 1, name: 'Info 2', max_team_size: 2 });
    getOwnTeam.mockResolvedValue({
      id: 3,
      join_code: 'K7Q2M9XD',
      locked: false,
      members: [
        { id: 112, first_name: 'Ada', last_name: 'Lovelace', email: 'ada@example.org' },
        { id: 113, first_name: 'Alan', last_name: 'Turing', email: 'alan@example.org' },
      ],
    });
    renderSheet();

    expect(await screen.findByText(/You hand in as a team with Ada Lovelace, Alan Turing/)).toBeInTheDocument();
  });

  it('tells students without a team that they hand in alone', async () => {
    courseGet.mockResolvedValue({ id: 1, name: 'Info 2', max_team_size: 2 });
    renderSheet();

    expect(await screen.findByText(/You are not in a team yet, so you hand in on your own/)).toBeInTheDocument();
  });

  it('shows no team notice in courses without teams', async () => {
    renderSheet();

    expect(await screen.findByText('task 0')).toBeInTheDocument();
    expect(screen.queryByText(/hand in/)).not.toBeInTheDocument();
    expect(getOwnTeam).not.toHaveBeenCalled();
  });
});
