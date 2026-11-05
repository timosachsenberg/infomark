// TeamsPanel: students form and join teams, tutors see every team, course
// admins additionally manage them. The API layer is fully mocked.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

const mocks = vi.hoisted(() => ({
  getOwn: vi.fn(),
  getAll: vi.fn(),
  create: vi.fn(),
  join: vi.fn(),
  leave: vi.fn(),
  getEnrollments: vi.fn(),
}));

vi.mock('@/api/agent', () => ({
  default: {
    Teams: {
      getOwn: mocks.getOwn,
      getAll: mocks.getAll,
      create: mocks.create,
      join: mocks.join,
      leave: mocks.leave,
      delete: vi.fn(),
      addMember: vi.fn(),
      removeMember: vi.fn(),
    },
    Courses: { getEnrollments: mocks.getEnrollments },
  },
}));

import '@/i18n';
import { TeamsPanel } from '@/components/course/TeamsPanel';

const ada = { id: 112, first_name: 'Ada', last_name: 'Lovelace', email: 'ada@example.org' };
const alan = { id: 113, first_name: 'Alan', last_name: 'Turing', email: 'alan@example.org' };
const grace = { id: 114, first_name: 'Grace', last_name: 'Hopper', email: 'grace@example.org' };

beforeEach(() => {
  vi.clearAllMocks();
  mocks.getOwn.mockRejectedValue({ status: 404, message: 'Not Found' });
  mocks.getAll.mockResolvedValue([]);
  mocks.getEnrollments.mockResolvedValue([]);
});

describe('TeamsPanel (student)', () => {
  it('lets a student without a team create one', async () => {
    mocks.create.mockResolvedValue({ id: 3, join_code: 'K7Q2M9XD', locked: false, members: [ada] });
    render(<TeamsPanel courseId={1} maxTeamSize={2} isStudent isTutor={false} canManage={false} />);

    expect(await screen.findByText('You are not in a team yet.')).toBeInTheDocument();

    mocks.getOwn.mockResolvedValue({ id: 3, join_code: 'K7Q2M9XD', locked: false, members: [ada] });
    await userEvent.click(screen.getByRole('button', { name: /Create team/ }));

    expect(mocks.create).toHaveBeenCalledWith(1);
    expect(await screen.findByText('K7Q2M9XD')).toBeInTheDocument();
    expect(screen.getByText('Ada Lovelace')).toBeInTheDocument();
  });

  it('lets a student join a team with a code', async () => {
    mocks.join.mockResolvedValue({ id: 3, join_code: 'K7Q2M9XD', locked: false, members: [ada, alan] });
    render(<TeamsPanel courseId={1} maxTeamSize={2} isStudent isTutor={false} canManage={false} />);

    const input = await screen.findByLabelText('Join code');
    await userEvent.type(input, 'k7q2m9xd ');
    await userEvent.click(screen.getByRole('button', { name: /Join team/ }));

    expect(mocks.join).toHaveBeenCalledWith(1, 'k7q2m9xd');
  });

  it('shows the team with its join code and a leave button', async () => {
    mocks.getOwn.mockResolvedValue({ id: 3, join_code: 'K7Q2M9XD', locked: false, members: [ada] });
    render(<TeamsPanel courseId={1} maxTeamSize={2} isStudent isTutor={false} canManage={false} />);

    expect(await screen.findByText('Ada Lovelace')).toBeInTheDocument();
    expect(screen.getByText('K7Q2M9XD')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Leave team/ })).toBeInTheDocument();
  });

  it('explains that a locked team cannot be changed', async () => {
    mocks.getOwn.mockResolvedValue({ id: 3, join_code: 'K7Q2M9XD', locked: true, members: [ada, alan] });
    render(<TeamsPanel courseId={1} maxTeamSize={2} isStudent isTutor={false} canManage={false} />);

    expect(await screen.findByText(/has handed in solutions/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Leave team/ })).not.toBeInTheDocument();
    expect(screen.queryByText('K7Q2M9XD')).not.toBeInTheDocument();
  });
});

describe('TeamsPanel (staff)', () => {
  const teams = [
    { id: 3, join_code: 'K7Q2M9XD', locked: true, members: [ada, alan] },
    { id: 4, join_code: 'P3RT8WQZ', locked: false, members: [grace] },
  ];

  it('shows tutors every team without admin controls', async () => {
    mocks.getAll.mockResolvedValue(teams);
    render(<TeamsPanel courseId={1} maxTeamSize={2} isStudent={false} isTutor canManage={false} />);

    expect(await screen.findByText('Ada Lovelace')).toBeInTheDocument();
    expect(screen.getByText('Grace Hopper')).toBeInTheDocument();
    expect(screen.getByText('Locked')).toBeInTheDocument();
    expect(screen.queryByText('K7Q2M9XD')).not.toBeInTheDocument();
    expect(screen.queryAllByLabelText('Select students')).toHaveLength(0);
    expect(mocks.getEnrollments).not.toHaveBeenCalled();
  });

  it('shows admins join codes and students without a team', async () => {
    mocks.getAll.mockResolvedValue(teams);
    mocks.getEnrollments.mockResolvedValue([
      { role: 0, user: ada },
      { role: 0, user: alan },
      { role: 0, user: grace },
      { role: 0, user: { id: 115, first_name: 'Edsger', last_name: 'Dijkstra', email: 'edsger@example.org' } },
      { role: 1, user: { id: 2, first_name: 'Tia', last_name: 'Tutor', email: 'tia@example.org' } },
    ]);
    render(<TeamsPanel courseId={1} maxTeamSize={2} isStudent={false} isTutor canManage />);

    expect(await screen.findByText('Students without a team: 1')).toBeInTheDocument();
    const row = screen.getByText('P3RT8WQZ').closest('tr') as HTMLElement;
    expect(within(row).getByRole('button', { name: 'Delete team' })).toBeInTheDocument();
    // antd labels both the select and its search input
    await waitFor(() => expect(screen.getAllByLabelText('Select students').length).toBeGreaterThan(0));
  });
});
