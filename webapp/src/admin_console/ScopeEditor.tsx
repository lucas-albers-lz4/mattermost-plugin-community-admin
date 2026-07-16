import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';

import type {PluginCustomSettingsComponentProps} from 'types/mattermost-webapp';

import {
    fetchAdminChannels,
    fetchAdminTeams,
    fetchAdminUsers,
    type AdminChannelOption,
    type AdminTeamOption,
    type AdminUserOption,
} from '../api';

type ScopeConfig = {
    version: number;
    email_domain: string;
    site_url: string;
    organizers: Array<{
        user_id: string;
        display_username: string;
        teams: Array<{id: string; name: string}>;
        channels: Array<{id: string; team_id: string; name: string}>;
        all_channels_in_teams: string[];
        permissions: Record<string, boolean>;
        rate_limits: {creates_per_hour: number; password_resets_per_hour: number};
    }>;
};

const emptyConfig = (): ScopeConfig => ({
    version: 1,
    email_domain: 'community.local',
    site_url: '',
    organizers: [],
});

const serializeConfig = (config: ScopeConfig): string => JSON.stringify(config, null, 2);

const userLabel = (u: AdminUserOption): string => {
    const full = [u.first_name, u.last_name].filter(Boolean).join(' ').trim();
    if (full && u.nickname) {
        return `${u.username} (${full} / ${u.nickname})`;
    }
    if (full) {
        return `${u.username} (${full})`;
    }
    if (u.nickname) {
        return `${u.username} (${u.nickname})`;
    }
    return u.username;
};

const selectedValues = (select: HTMLSelectElement): string[] =>
    Array.from(select.selectedOptions).map((o) => o.value);

const ScopeEditor: React.FC<PluginCustomSettingsComponentProps<string>> = (props) => {
    const propsRef = useRef(props);
    propsRef.current = props;

    const [cfg, setCfg] = useState<ScopeConfig>(() => {
        try {
            return props.value ? JSON.parse(props.value) : emptyConfig();
        } catch {
            return emptyConfig();
        }
    });

    const [teams, setTeams] = useState<AdminTeamOption[]>([]);
    const [channels, setChannels] = useState<AdminChannelOption[]>([]);
    const [users, setUsers] = useState<AdminUserOption[]>([]);
    const [userTerm, setUserTerm] = useState('');
    const [selectedUserId, setSelectedUserId] = useState('');
    const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([]);
    const [selectedChannelIds, setSelectedChannelIds] = useState<string[]>([]);
    const [message, setMessage] = useState('');
    const [loadingOptions, setLoadingOptions] = useState(false);

    const notifyChange = useCallback((value: string) => {
        const current = propsRef.current;
        current.onChange(current.id, value);
        current.setSaveNeeded();
    }, []);

    const persist = useCallback((next: ScopeConfig) => {
        const value = serializeConfig(next);
        setCfg(next);
        notifyChange(value);
    }, [notifyChange]);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            setLoadingOptions(true);
            try {
                const teamList = await fetchAdminTeams();
                if (!cancelled) {
                    setTeams(teamList);
                }
            } catch (e) {
                if (!cancelled) {
                    setMessage((e as Error).message);
                }
            } finally {
                if (!cancelled) {
                    setLoadingOptions(false);
                }
            }
        })();
        return () => {
            cancelled = true;
        };
    }, []);

    useEffect(() => {
        let cancelled = false;
        const handle = window.setTimeout(() => {
            (async () => {
                try {
                    const list = await fetchAdminUsers(userTerm);
                    if (!cancelled) {
                        setUsers(list);
                    }
                } catch (e) {
                    if (!cancelled) {
                        setMessage((e as Error).message);
                    }
                }
            })();
        }, 250);
        return () => {
            cancelled = true;
            window.clearTimeout(handle);
        };
    }, [userTerm]);

    useEffect(() => {
        if (selectedTeamIds.length === 0) {
            setChannels([]);
            setSelectedChannelIds([]);
            return () => {
                // no-op cleanup when no teams selected
            };
        }
        let cancelled = false;
        (async () => {
            try {
                const lists = await Promise.all(selectedTeamIds.map((id) => fetchAdminChannels(id)));
                if (cancelled) {
                    return;
                }
                const merged: AdminChannelOption[] = [];
                const seen = new Set<string>();
                for (const list of lists) {
                    for (const ch of list) {
                        if (seen.has(ch.id)) {
                            continue;
                        }
                        seen.add(ch.id);
                        merged.push(ch);
                    }
                }
                setChannels(merged);
                setSelectedChannelIds((prev) => prev.filter((id) => seen.has(id)));
            } catch (e) {
                if (!cancelled) {
                    setMessage((e as Error).message);
                }
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [selectedTeamIds]);

    const selectedUser = useMemo(
        () => users.find((u) => u.id === selectedUserId),
        [users, selectedUserId],
    );

    const addOrganizer = () => {
        setMessage('');
        if (!selectedUserId || !selectedUser) {
            setMessage('Select an organizer user');
            return;
        }
        if (selectedTeamIds.length === 0) {
            setMessage('Select at least one team (otherwise Create User has an empty team dropdown)');
            return;
        }

        const resolvedTeams = selectedTeamIds.map((id) => {
            const t = teams.find((x) => x.id === id);
            return {
                id,
                name: t?.display_name || t?.name || id,
            };
        });

        const resolvedChannels = selectedChannelIds.map((id) => {
            const ch = channels.find((x) => x.id === id);
            return {
                id,
                team_id: ch?.team_id || '',
                name: ch?.display_name || ch?.name || id,
            };
        });

        const allChannelsInTeams = resolvedChannels.length === 0 ? resolvedTeams.map((t) => t.id) : [];

        const next = {...cfg};
        next.organizers = next.organizers.filter((o) => o.user_id !== selectedUser.id);
        next.organizers.push({
            user_id: selectedUser.id,
            display_username: selectedUser.username,
            teams: resolvedTeams,
            channels: resolvedChannels,
            all_channels_in_teams: allChannelsInTeams,
            permissions: {
                create_user: true,
                edit_profile: true,
                reset_password: true,
                manage_membership: true,
                remove_from_team: true,
                deactivate_globally: false,
            },
            rate_limits: {creates_per_hour: 20, password_resets_per_hour: 10},
        });
        persist(next);
        setSelectedUserId('');
        setUserTerm('');
        setSelectedTeamIds([]);
        setSelectedChannelIds([]);
        setMessage('Organizer added. Save plugin settings to apply.');
    };

    return (
        <div>
            <p>{props.helpText}</p>
            <label>
                {'Site URL'}
                <input
                    value={cfg.site_url}
                    onChange={(e) => persist({...cfg, site_url: e.target.value})}
                    style={{display: 'block', width: '100%', marginBottom: 8}}
                    disabled={props.disabled}
                />
            </label>
            <label>
                {'Email domain'}
                <input
                    value={cfg.email_domain}
                    onChange={(e) => persist({...cfg, email_domain: e.target.value})}
                    style={{display: 'block', width: '100%', marginBottom: 8}}
                    disabled={props.disabled}
                />
            </label>

            <h4>{'Add organizer'}</h4>
            {loadingOptions && <div style={{marginBottom: 8}}>{'Loading teams and users…'}</div>}

            <label style={{display: 'block', marginBottom: 8}}>
                {'Filter users'}
                <input
                    placeholder='type to search username…'
                    value={userTerm}
                    onChange={(e) => setUserTerm(e.target.value)}
                    style={{display: 'block', width: '100%', marginTop: 4}}
                    disabled={props.disabled}
                />
            </label>
            <label style={{display: 'block', marginBottom: 8}}>
                {'Organizer'}
                <select
                    value={selectedUserId}
                    onChange={(e) => setSelectedUserId(e.target.value)}
                    style={{display: 'block', width: '100%', marginTop: 4}}
                    disabled={props.disabled}
                >
                    <option value=''>{'Select user'}</option>
                    {users.map((u) => (
                        <option
                            key={u.id}
                            value={u.id}
                        >
                            {userLabel(u)}
                        </option>
                    ))}
                </select>
            </label>

            <label style={{display: 'block', marginBottom: 8}}>
                {'Teams (required — hold Cmd/Ctrl to multi-select)'}
                <select
                    multiple={true}
                    size={Math.min(8, Math.max(3, teams.length || 3))}
                    value={selectedTeamIds}
                    onChange={(e) => setSelectedTeamIds(selectedValues(e.target))}
                    style={{display: 'block', width: '100%', marginTop: 4}}
                    disabled={props.disabled}
                >
                    {teams.map((t) => (
                        <option
                            key={t.id}
                            value={t.id}
                        >
                            {t.display_name || t.name}
                        </option>
                    ))}
                </select>
            </label>

            <label style={{display: 'block', marginBottom: 8}}>
                {'Channels (optional — leave empty for all public channels in selected teams)'}
                <select
                    multiple={true}
                    size={Math.min(8, Math.max(3, channels.length || 3))}
                    value={selectedChannelIds}
                    onChange={(e) => setSelectedChannelIds(selectedValues(e.target))}
                    style={{display: 'block', width: '100%', marginTop: 4}}
                    disabled={props.disabled || selectedTeamIds.length === 0}
                >
                    {channels.map((ch) => (
                        <option
                            key={ch.id}
                            value={ch.id}
                        >
                            {ch.display_name || ch.name}
                        </option>
                    ))}
                </select>
            </label>

            <button
                type='button'
                onClick={addOrganizer}
                disabled={props.disabled}
            >
                {'Add organizer'}
            </button>
            {message && <div style={{marginTop: 8}}>{message}</div>}

            <h4 style={{marginTop: 16}}>
                {'Current organizers ('}
                {cfg.organizers.length}
                {')'}
            </h4>
            <pre style={{maxHeight: 240, overflow: 'auto', background: '#f8f8f8', padding: 8}}>
                {JSON.stringify(cfg.organizers, null, 2)}
            </pre>
            <h4>{'Raw JSON'}</h4>
            <textarea
                value={serializeConfig(cfg)}
                onChange={(e) => {
                    const {value} = e.target;
                    try {
                        setCfg(JSON.parse(value));
                    } catch {
                        // Allow invalid JSON while editing; still mark settings dirty.
                    }
                    notifyChange(value);
                }}
                rows={12}
                style={{width: '100%', fontFamily: 'monospace'}}
                disabled={props.disabled}
            />
        </div>
    );
};

export default ScopeEditor;
