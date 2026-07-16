import React, {useEffect, useMemo, useState} from 'react';

import type {MeResponse, UserSummary} from '../api';
import {pluginFetch} from '../api';

type Props = {
    onClose: () => void;
};

const CommunityPanel: React.FC<Props> = ({onClose}) => {
    const [me, setMe] = useState<MeResponse | null>(null);
    const [users, setUsers] = useState<UserSummary[]>([]);
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState('');
    const [showCreate, setShowCreate] = useState(false);
    const [credentialText, setCredentialText] = useState('');
    const [form, setForm] = useState({username: '', first_name: '', last_name: '', team_id: '', channel_id: ''});

    const load = async () => {
        setLoading(true);
        setError('');
        try {
            const meResp = await pluginFetch<MeResponse>('/me');
            setMe(meResp);
            const usersResp = await pluginFetch<{users: UserSummary[]}>('/users?q=' + encodeURIComponent(search));
            setUsers(usersResp.users || []);
        } catch (e) {
            setError((e as Error).message);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        load();
    }, []);

    const channelsForTeam = useMemo(() => {
        const all = me?.channels || [];
        if (!form.team_id) {
            return all;
        }
        return all.filter((c) => !c.team_id || c.team_id === form.team_id);
    }, [me?.channels, form.team_id]);

    const createUser = async () => {
        setError('');
        if (!form.username.trim()) {
            setError('Username is required');
            return;
        }
        if (!form.team_id) {
            setError('Select a team so the new user can log in');
            return;
        }
        try {
            const teamIds = [form.team_id];
            const channelIds = form.channel_id ? [form.channel_id] : [];
            const result = await pluginFetch<{user: UserSummary; password: string; parent_text: string}>('/users', {
                method: 'POST',
                body: JSON.stringify({
                    username: form.username,
                    first_name: form.first_name,
                    last_name: form.last_name,
                    team_ids: teamIds,
                    channel_ids: channelIds,
                }),
            });
            setCredentialText(result.parent_text);
            setShowCreate(false);
            setForm({username: '', first_name: '', last_name: '', team_id: '', channel_id: ''});
            await load();
        } catch (e) {
            setError((e as Error).message);
        }
    };

    const resetPassword = async (userId: string) => {
        setError('');
        try {
            const result = await pluginFetch<{parent_text: string}>('/users/' + userId + '/reset-password', {method: 'POST'});
            setCredentialText(result.parent_text);
        } catch (e) {
            setError((e as Error).message);
        }
    };

    const removeFromTeam = async (userId: string, teamId: string) => {
        setError('');
        try {
            await pluginFetch('/users/' + userId + '/teams/' + teamId, {method: 'DELETE'});
            await load();
        } catch (e) {
            setError((e as Error).message);
        }
    };

    if (loading && !me) {
        return (
            <div
                data-testid='community-admin-loading'
                style={{padding: 16}}
            >
                {'Loading community admin...'}
            </div>
        );
    }

    if (error && !me) {
        return null;
    }

    const teams = me?.teams || [];
    const hasTeams = teams.length > 0;

    return (
        <div
            data-testid='community-admin-panel'
            style={{padding: 16, maxWidth: 900, margin: '0 auto'}}
        >
            <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
                <h2>{'Community Members'}</h2>
                <button
                    type='button'
                    data-testid='community-admin-close'
                    onClick={onClose}
                >
                    {'Close'}
                </button>
            </div>
            {error && (
                <div
                    data-testid='community-admin-error'
                    style={{color: 'crimson', marginBottom: 12}}
                >
                    {error}
                </div>
            )}
            <div style={{display: 'flex', gap: 8, marginBottom: 12}}>
                <input
                    data-testid='community-admin-search'
                    placeholder='Search users'
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    style={{flex: 1}}
                />
                <button
                    type='button'
                    data-testid='community-admin-search-btn'
                    onClick={load}
                >
                    {'Search'}
                </button>
                {me?.permissions.create_user && (
                    <button
                        type='button'
                        data-testid='community-admin-create-toggle'
                        onClick={() => setShowCreate(!showCreate)}
                    >
                        {'Create user'}
                    </button>
                )}
            </div>
            {showCreate && (
                <div
                    data-testid='community-admin-create-form'
                    style={{border: '1px solid #ccc', padding: 12, marginBottom: 12}}
                >
                    <h3>{'Create user'}</h3>
                    {!hasTeams && (
                        <div
                            data-testid='community-admin-no-teams'
                            style={{color: 'crimson', marginBottom: 8}}
                        >
                            {'No teams in your organizer scope. Ask a system admin to add teams under System Console → Plugins → Community Admin, then save.'}
                        </div>
                    )}
                    <div style={{display: 'grid', gap: 8}}>
                        <input
                            data-testid='community-admin-create-username'
                            placeholder='username'
                            value={form.username}
                            onChange={(e) => setForm({...form, username: e.target.value})}
                        />
                        <input
                            data-testid='community-admin-create-firstname'
                            placeholder='first name'
                            value={form.first_name}
                            onChange={(e) => setForm({...form, first_name: e.target.value})}
                        />
                        <input
                            data-testid='community-admin-create-lastname'
                            placeholder='last name'
                            value={form.last_name}
                            onChange={(e) => setForm({...form, last_name: e.target.value})}
                        />
                        <select
                            data-testid='community-admin-create-team'
                            value={form.team_id}
                            onChange={(e) => setForm({...form, team_id: e.target.value, channel_id: ''})}
                        >
                            <option value=''>{'Select team (required)'}</option>
                            {teams.map((t) => (
                                <option
                                    key={t.id}
                                    value={t.id}
                                >
                                    {t.name}
                                </option>
                            ))}
                        </select>
                        <select
                            data-testid='community-admin-create-channel'
                            value={form.channel_id}
                            onChange={(e) => setForm({...form, channel_id: e.target.value})}
                            disabled={!form.team_id}
                        >
                            <option value=''>
                                {form.team_id ? 'Select channel (optional)' : 'Select a team first'}
                            </option>
                            {channelsForTeam.map((c) => (
                                <option
                                    key={c.id}
                                    value={c.id}
                                >
                                    {c.name}
                                </option>
                            ))}
                        </select>
                        <button
                            type='button'
                            data-testid='community-admin-create-submit'
                            onClick={createUser}
                            disabled={!hasTeams}
                        >
                            {'Create'}
                        </button>
                    </div>
                </div>
            )}
            {credentialText && (
                <div
                    data-testid='community-admin-credentials'
                    style={{background: '#f6f6f6', padding: 12, marginBottom: 12}}
                >
                    <strong>{'Credential handoff (copy now — not stored):'}</strong>
                    <pre
                        data-testid='community-admin-credentials-text'
                        style={{whiteSpace: 'pre-wrap'}}
                    >
                        {credentialText}
                    </pre>
                    <button
                        type='button'
                        onClick={() => navigator.clipboard.writeText(credentialText)}
                    >
                        {'Copy'}
                    </button>
                    <button
                        type='button'
                        onClick={() => setCredentialText('')}
                        style={{marginLeft: 8}}
                    >
                        {'Dismiss'}
                    </button>
                </div>
            )}
            <table
                data-testid='community-admin-user-table'
                style={{width: '100%', borderCollapse: 'collapse'}}
            >
                <thead>
                    <tr>
                        <th align='left'>{'Username'}</th>
                        <th align='left'>{'Name'}</th>
                        <th align='left'>{'Actions'}</th>
                    </tr>
                </thead>
                <tbody>
                    {users.map((u) => (
                        <tr
                            key={u.id}
                            data-testid={`community-admin-user-row-${u.username}`}
                            style={{borderTop: '1px solid #ddd'}}
                        >
                            <td>{u.username}</td>
                            <td>{u.first_name} {u.last_name}</td>
                            <td>
                                {me?.permissions.reset_password && (
                                    <button
                                        type='button'
                                        data-testid={`community-admin-reset-${u.username}`}
                                        onClick={() => resetPassword(u.id)}
                                        style={{marginRight: 8}}
                                    >
                                        {'Reset password'}
                                    </button>
                                )}
                                {me?.permissions.remove_from_team && me?.teams?.[0] && (
                                    <button
                                        type='button'
                                        data-testid={`community-admin-remove-${u.username}`}
                                        onClick={() => removeFromTeam(u.id, me.teams[0].id)}
                                    >
                                        {'Remove from '}
                                        {me.teams[0].name}
                                    </button>
                                )}
                            </td>
                        </tr>
                    ))}
                </tbody>
            </table>
            <p style={{marginTop: 16, color: '#666'}}>
                {'Mobile organizers: use /community-admin reset-password USERNAME or remove-from-team USERNAME TEAM'}
            </p>
        </div>
    );
};

export default CommunityPanel;
