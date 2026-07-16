import React, {useCallback, useRef, useState} from 'react';

import type {PluginCustomSettingsComponentProps} from 'types/mattermost-webapp';

import {pluginFetch} from '../api';

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
    const [draftUsername, setDraftUsername] = useState('');
    const [draftTeams, setDraftTeams] = useState('');
    const [draftChannels, setDraftChannels] = useState('');
    const [message, setMessage] = useState('');

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

    const addOrganizer = async () => {
        setMessage('');
        const teamNames = draftTeams.split(',').map((s) => s.trim()).filter(Boolean);
        if (!draftUsername.trim()) {
            setMessage('Organizer username required');
            return;
        }
        if (teamNames.length === 0) {
            setMessage('At least one team name is required (otherwise Create User has an empty team dropdown)');
            return;
        }
        try {
            const resolved = await pluginFetch<{
                organizer?: {user_id: string; username: string};
                teams?: Array<{id: string; name: string}>;
                channels?: Array<{id: string; team_id: string; name: string}>;
            }>('/resolve-scope', {
                method: 'POST',
                body: JSON.stringify({
                    organizer_username: draftUsername.trim(),
                    team_names: teamNames,
                    channel_specs: draftChannels.split(',').map((s) => s.trim()).filter(Boolean),
                }),
            });

            if (!resolved.organizer) {
                setMessage('Organizer username required');
                return;
            }
            if (!(resolved.teams || []).length) {
                setMessage('No teams resolved — check team names (use the team URL slug, not display name)');
                return;
            }

            const next = {...cfg};
            next.organizers = next.organizers.filter((o) => o.user_id !== resolved.organizer!.user_id);
            next.organizers.push({
                user_id: resolved.organizer.user_id,
                display_username: resolved.organizer.username,
                teams: resolved.teams || [],
                channels: resolved.channels || [],
                // When no explicit channels are listed, allow all public channels in scoped teams.
                all_channels_in_teams: (resolved.channels || []).length === 0
                    ? (resolved.teams || []).map((t) => t.id)
                    : [],
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
            setDraftUsername('');
            setDraftTeams('');
            setDraftChannels('');
            setMessage('Organizer added. Save plugin settings to apply.');
        } catch (e) {
            setMessage((e as Error).message);
        }
    };

    return (
        <div>
            <p>{props.helpText}</p>
            <label>
                Site URL
                <input
                    value={cfg.site_url}
                    onChange={(e) => persist({...cfg, site_url: e.target.value})}
                    style={{display: 'block', width: '100%', marginBottom: 8}}
                />
            </label>
            <label>
                Email domain
                <input
                    value={cfg.email_domain}
                    onChange={(e) => persist({...cfg, email_domain: e.target.value})}
                    style={{display: 'block', width: '100%', marginBottom: 8}}
                />
            </label>
            <h4>Add organizer</h4>
            <input placeholder='organizer username' value={draftUsername} onChange={(e) => setDraftUsername(e.target.value)} style={{width: '100%', marginBottom: 8}}/>
            <input placeholder='teams (comma-separated URL names/slugs, required)' value={draftTeams} onChange={(e) => setDraftTeams(e.target.value)} style={{width: '100%', marginBottom: 8}}/>
            <input placeholder='channels optional (team-slug:channel-slug, comma-separated); leave blank for all public channels in those teams' value={draftChannels} onChange={(e) => setDraftChannels(e.target.value)} style={{width: '100%', marginBottom: 8}}/>
            <button type='button' onClick={addOrganizer} disabled={props.disabled}>Resolve and add</button>
            {message && <div style={{marginTop: 8}}>{message}</div>}
            <h4 style={{marginTop: 16}}>Current organizers ({cfg.organizers.length})</h4>
            <pre style={{maxHeight: 240, overflow: 'auto', background: '#f8f8f8', padding: 8}}>{JSON.stringify(cfg.organizers, null, 2)}</pre>
            <h4>Raw JSON</h4>
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
