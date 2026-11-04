import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Card,
  Empty,
  Input,
  List,
  Modal,
  Popconfirm,
  Select,
  Skeleton,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { DeleteOutlined, LockOutlined, LogoutOutlined, PlusOutlined, TeamOutlined, UserOutlined } from '@ant-design/icons';
import agent from '@/api/agent';
import type { ApiError, Enrollment, Team, TeamMember } from '@/types';
import { useTranslation } from 'react-i18next';

interface TeamsPanelProps {
  courseId: number;
  maxTeamSize: number;
  isStudent: boolean;
  isTutor: boolean;
  // Course admins can create, change and delete every team.
  canManage: boolean;
}

const fullName = (m: TeamMember) => `${m.first_name} ${m.last_name}`;

// TeamsPanel shows the fixed teams of a course. Students create a team and
// share its join code or join a team with a code; once a team has handed in a
// solution it is locked for students. Tutors see all teams, course admins can
// additionally build teams and add or remove members.
export const TeamsPanel: React.FC<TeamsPanelProps> = ({ courseId, maxTeamSize, isStudent, isTutor, canManage }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [ownTeam, setOwnTeam] = useState<Team | null>(null);
  const [joinCode, setJoinCode] = useState('');
  const [teams, setTeams] = useState<Team[]>([]);
  const [students, setStudents] = useState<Enrollment[]>([]);
  const [newTeamMembers, setNewTeamMembers] = useState<number[]>([]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      if (isStudent) {
        // 404 simply means "not in a team yet".
        setOwnTeam(await agent.Teams.getOwn(courseId).catch(() => null));
      }
      if (isTutor) {
        setTeams(await agent.Teams.getAll(courseId));
      }
      if (canManage) {
        const enrollments = await agent.Courses.getEnrollments(courseId);
        setStudents(enrollments.filter((e) => e.role === 0));
      }
    } catch {
      message.error(t('messages.updateFailed'));
    } finally {
      setLoading(false);
    }
  }, [courseId, isStudent, isTutor, canManage, t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Students who are not in any team yet (admins pick from these).
  const studentsWithoutTeam = useMemo(() => {
    const inTeam = new Set(teams.flatMap((team) => team.members.map((m) => m.id)));
    return students.filter((s) => !inTeam.has(s.user.id));
  }, [teams, students]);

  const studentOptions = studentsWithoutTeam.map((s) => ({
    value: s.user.id,
    label: `${s.user.first_name} ${s.user.last_name} (${s.user.email})`,
  }));

  // Runs a team action, reports the outcome and reloads the panel.
  const run = async (action: () => Promise<unknown>, success: string, failure?: (error: ApiError) => string) => {
    setBusy(true);
    try {
      await action();
      message.success(success);
      await fetchData();
    } catch (error) {
      const apiError = error as ApiError;
      message.error(failure?.(apiError) ?? apiError?.message ?? t('team.actionFailed'));
    } finally {
      setBusy(false);
    }
  };

  const handleCreateOwn = () => run(() => agent.Teams.create(courseId), t('team.created'));

  const handleJoin = () =>
    run(
      async () => {
        await agent.Teams.join(courseId, joinCode.trim());
        setJoinCode('');
      },
      t('team.joined'),
      (error) => (error.status === 404 ? t('team.notFound') : error.message),
    );

  const handleLeave = () => run(() => agent.Teams.leave(courseId), t('team.left'));

  const handleCreateForStudents = () =>
    run(async () => {
      await agent.Teams.create(courseId, newTeamMembers);
      setNewTeamMembers([]);
    }, t('team.created'));

  const handleAddMember = (teamId: number, userId: number) =>
    run(() => agent.Teams.addMember(courseId, teamId, userId), t('team.memberAdded'));

  const confirmRemoveMember = (team: Team, member: TeamMember) =>
    Modal.confirm({
      title: t('team.removeMember'),
      content: t('team.removeConfirm', { name: fullName(member) }),
      okButtonProps: { danger: true },
      okText: t('team.removeMember'),
      cancelText: t('common.cancel'),
      onOk: () => run(() => agent.Teams.removeMember(courseId, team.id, member.id), t('team.memberRemoved')),
    });

  const handleDelete = (teamId: number) => run(() => agent.Teams.delete(courseId, teamId), t('team.deleted'));

  if (loading) return <Skeleton active paragraph={{ rows: 4 }} />;

  const studentView = isStudent && (
    <Card
      title={
        <Space>
          <TeamOutlined />
          {t('team.myTeam')}
          {ownTeam?.locked && (
            <Tag icon={<LockOutlined />} color="default">
              {t('team.locked')}
            </Tag>
          )}
        </Space>
      }
      style={{ marginBottom: isTutor ? 16 : 0 }}
    >
      {ownTeam ? (
        <Space direction="vertical" style={{ width: '100%' }}>
          <List
            size="small"
            dataSource={ownTeam.members}
            renderItem={(member) => (
              <List.Item>
                <List.Item.Meta avatar={<UserOutlined />} title={fullName(member)} description={member.email} />
              </List.Item>
            )}
          />
          {ownTeam.locked ? (
            <Alert type="info" showIcon message={t('team.lockedHint')} />
          ) : (
            <>
              {ownTeam.members.length < maxTeamSize && (
                <Space direction="vertical" size={0}>
                  <Space>
                    <span>{t('team.joinCode')}:</span>
                    <Typography.Text code copyable strong>
                      {ownTeam.join_code}
                    </Typography.Text>
                  </Space>
                  <Typography.Text type="secondary">{t('team.joinCodeHint')}</Typography.Text>
                </Space>
              )}
              <Popconfirm
                title={t('team.leaveConfirm')}
                okText={t('team.leave')}
                cancelText={t('common.cancel')}
                onConfirm={handleLeave}
              >
                <Button danger icon={<LogoutOutlined />} loading={busy}>
                  {t('team.leave')}
                </Button>
              </Popconfirm>
            </>
          )}
        </Space>
      ) : (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Alert type="info" showIcon message={t('team.noTeam')} description={t('team.formTeamHint', { count: maxTeamSize })} />
          <Space wrap>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreateOwn} loading={busy}>
              {t('team.create')}
            </Button>
            <Space.Compact>
              <Input
                value={joinCode}
                onChange={(e) => setJoinCode(e.target.value)}
                onPressEnter={() => joinCode.trim() && handleJoin()}
                placeholder={t('team.joinPlaceholder')}
                aria-label={t('team.joinCode')}
                style={{ width: 160 }}
              />
              <Button onClick={handleJoin} disabled={!joinCode.trim()} loading={busy}>
                {t('team.join')}
              </Button>
            </Space.Compact>
          </Space>
        </Space>
      )}
    </Card>
  );

  const columns = [
    {
      title: t('team.team'),
      key: 'team',
      render: (_: unknown, team: Team) => (
        <Space wrap>
          {t('team.teamNumber', { id: team.id })}
          {team.locked && <Tag icon={<LockOutlined />}>{t('team.locked')}</Tag>}
          {team.members.length >= maxTeamSize && <Tag color="blue">{t('team.full')}</Tag>}
        </Space>
      ),
    },
    {
      title: t('team.members'),
      key: 'members',
      render: (_: unknown, team: Team) => (
        <Space wrap size={[4, 4]}>
          {team.members.map((member) => (
            <Tag
              key={member.id}
              closable={canManage}
              onClose={(e) => {
                e.preventDefault();
                confirmRemoveMember(team, member);
              }}
            >
              {fullName(member)}
            </Tag>
          ))}
        </Space>
      ),
    },
    ...(canManage
      ? [
          {
            title: t('team.joinCode'),
            key: 'join_code',
            render: (_: unknown, team: Team) => <Typography.Text code>{team.join_code}</Typography.Text>,
          },
          {
            title: t('common.actions'),
            key: 'actions',
            render: (_: unknown, team: Team) => (
              <Space wrap>
                {team.members.length < maxTeamSize && (
                  <Select
                    showSearch
                    size="small"
                    style={{ width: 200 }}
                    placeholder={t('team.addMember')}
                    aria-label={t('team.addMember')}
                    options={studentOptions}
                    optionFilterProp="label"
                    value={null}
                    onChange={(userId: number) => handleAddMember(team.id, userId)}
                    disabled={busy}
                  />
                )}
                <Popconfirm
                  title={t('team.deleteConfirm')}
                  okText={t('common.delete')}
                  okButtonProps={{ danger: true }}
                  cancelText={t('common.cancel')}
                  onConfirm={() => handleDelete(team.id)}
                >
                  <Button size="small" danger icon={<DeleteOutlined />} aria-label={t('team.deleteTeam')} />
                </Popconfirm>
              </Space>
            ),
          },
        ]
      : []),
  ];

  const staffView = isTutor && (
    <Card
      title={
        <Space>
          <TeamOutlined />
          {`${t('course.teams')} (${teams.length})`}
        </Space>
      }
      extra={canManage && <Typography.Text type="secondary">{t('team.studentsWithoutTeam', { count: studentsWithoutTeam.length })}</Typography.Text>}
    >
      {canManage && (
        <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
          <Select
            mode="multiple"
            showSearch
            style={{ width: '100%' }}
            placeholder={t('team.selectStudents')}
            aria-label={t('team.selectStudents')}
            options={studentOptions}
            optionFilterProp="label"
            value={newTeamMembers}
            maxCount={maxTeamSize}
            onChange={setNewTeamMembers}
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleCreateForStudents}
            disabled={newTeamMembers.length === 0}
            loading={busy}
          >
            {t('team.create')}
          </Button>
        </Space.Compact>
      )}
      {teams.length === 0 ? (
        <Empty description={t('team.noTeams')} />
      ) : (
        <Table
          dataSource={teams}
          columns={columns}
          rowKey={(team) => team.id}
          pagination={{ pageSize: 20, hideOnSinglePage: true }}
          scroll={{ x: 'max-content' }}
        />
      )}
    </Card>
  );

  return (
    <div>
      {studentView}
      {staffView}
    </div>
  );
};

export default TeamsPanel;
