import React, {useState} from 'react';
import type {Store} from 'redux';
import type {UnknownAction} from 'redux';
import type {GlobalState} from '@mattermost/types/store';
import type {PluginRegistry} from 'types/mattermost-webapp';
import manifest from 'manifest';

import CommunityPanel from './components/CommunityPanel';
import ScopeEditor from './admin_console/ScopeEditor';
import {pluginFetch, MeResponse} from './api';

type PanelWrapperProps = {
    onClose: () => void;
};

const MenuIcon = () => <i className='icon fa fa-users'/>;

type ExtendedRegistry = PluginRegistry & {
    registerProductSwitcherMenuItem?: (
        text: string,
        icon: React.ReactNode,
        action: () => void,
        isHidden?: () => boolean,
    ) => string;
};

const PanelWrapper: React.FC<PanelWrapperProps> = ({onClose}) => {
    const [allowed, setAllowed] = useState<boolean | null>(null);

    React.useEffect(() => {
        pluginFetch<MeResponse>('/me').then(() => setAllowed(true)).catch(() => setAllowed(false));
    }, []);

    if (allowed === null || !allowed) {
        return null;
    }
    return <CommunityPanel onClose={onClose}/>;
};

export default class Plugin {
    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        registry.registerAdminConsoleCustomSetting('ScopeConfig', ScopeEditor);

        const {showRHSPlugin, hideRHSPlugin} = registry.registerRightHandSidebarComponent(
            () => <PanelWrapper onClose={() => store.dispatch(hideRHSPlugin as unknown as UnknownAction)}/>,
            'Community Members',
        );

        const openPanel = () => store.dispatch(showRHSPlugin as unknown as UnknownAction);

        // Exposed for Playwright e2e when Entry Edition omits plugin menu items.
        // Opening the RHS still requires a successful /me organizer check in PanelWrapper.
        window.__communityAdminOpenPanel = openPanel;

        registry.registerChannelHeaderButtonAction(
            <MenuIcon/>,
            openPanel,
            'Community Members',
            'Community Members',
        );

        registry.registerMainMenuAction(
            'Community Members',
            openPanel,
            'Community Members',
        );

        const extendedRegistry = registry as ExtendedRegistry;
        if (extendedRegistry.registerProductSwitcherMenuItem) {
            extendedRegistry.registerProductSwitcherMenuItem(
                'Community Members',
                <MenuIcon/>,
                openPanel,
            );
        }
    }
}

declare global {
    interface Window {
        registerPlugin: (pluginId: string, plugin: Plugin) => void;
        __communityAdminOpenPanel?: () => void;
    }
}

window.registerPlugin(manifest.id, new Plugin());
