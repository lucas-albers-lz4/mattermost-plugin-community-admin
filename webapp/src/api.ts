import manifest from 'manifest';

export const pluginId = manifest.id;

export async function pluginFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(`/plugins/${pluginId}/api/v1${path}`, {
        headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest',
            ...(options.headers || {}),
        },
        credentials: 'same-origin',
        ...options,
    });

    if (!response.ok) {
        let message = response.statusText;
        try {
            const body = await response.json();
            message = body.error || message;
        } catch {
            // ignore
        }
        throw new Error(message);
    }

    if (response.status === 204) {
        return {} as T;
    }
    return response.json() as Promise<T>;
}

export type MeResponse = {
    user_id: string;
    display_username: string;
    teams: Array<{id: string; name: string}>;
    channels: Array<{id: string; team_id: string; name: string}>;
    permissions: {
        create_user: boolean;
        edit_profile: boolean;
        reset_password: boolean;
        manage_membership: boolean;
        remove_from_team: boolean;
        deactivate_globally: boolean;
    };
    site_url: string;
};

export type UserSummary = {
    id: string;
    username: string;
    first_name: string;
    last_name: string;
    email: string;
    delete_at: number;
};

export type AdminUserOption = {
    id: string;
    username: string;
    nickname: string;
    first_name: string;
    last_name: string;
};

export type AdminTeamOption = {
    id: string;
    name: string;
    display_name: string;
};

export type AdminChannelOption = {
    id: string;
    team_id: string;
    name: string;
    display_name: string;
};

export async function fetchAdminUsers(term = '', page = 0, perPage = 50): Promise<AdminUserOption[]> {
    const params = new URLSearchParams({
        page: String(page),
        per_page: String(perPage),
    });
    if (term.trim()) {
        params.set('term', term.trim());
    }
    const res = await pluginFetch<{users: AdminUserOption[]}>(`/admin/users?${params.toString()}`);
    return res.users || [];
}

export async function fetchAdminTeams(page = 0, perPage = 200): Promise<AdminTeamOption[]> {
    const params = new URLSearchParams({
        page: String(page),
        per_page: String(perPage),
    });
    const res = await pluginFetch<{teams: AdminTeamOption[]}>(`/admin/teams?${params.toString()}`);
    return res.teams || [];
}

export async function fetchAdminChannels(teamId: string): Promise<AdminChannelOption[]> {
    const res = await pluginFetch<{channels: AdminChannelOption[]}>(
        `/admin/teams/${encodeURIComponent(teamId)}/channels`,
    );
    return res.channels || [];
}
