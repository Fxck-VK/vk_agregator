import { useEffect, useState } from 'react';
import bridge from '@vkontakte/vk-bridge';
import {
  AdaptivityProvider,
  AppRoot,
  ConfigProvider,
  Epic,
  Tabbar,
  TabbarItem,
  View,
  Panel,
} from '@vkontakte/vkui';
import { Icon28NewsfeedOutline, Icon28WalletOutline } from '@vkontakte/icons';
import { setLaunchParams } from './api';
import { JobsPanel } from './panels/JobsPanel';
import { NewJobPanel } from './panels/NewJobPanel';
import { JobDetailPanel } from './panels/JobDetailPanel';
import { BalancePanel } from './panels/BalancePanel';

type ActiveStory = 'jobs' | 'balance';
type ActivePanel = 'jobs' | 'new_job' | 'job_detail' | 'balance';

export default function App() {
  const [activeStory, setActiveStory] = useState<ActiveStory>('jobs');
  const [activePanel, setActivePanel] = useState<ActivePanel>('jobs');
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    // Initialize VK Bridge and capture launch params.
    bridge.send('VKWebAppInit').catch(() => {
      // Running outside VK (local dev) – safe to ignore.
    });

    // Launch params are the URL query string passed by VK.
    const raw = window.location.search.slice(1);
    if (raw) {
      setLaunchParams(raw);
    }
    setInitialized(true);
  }, []);

  if (!initialized) {
    return null;
  }

  const handleOpenNewJob = () => {
    setActivePanel('new_job');
    setActiveStory('jobs');
  };

  const handleJobCreated = (jobId: string) => {
    setSelectedJobId(jobId);
    setActivePanel('job_detail');
  };

  const handleViewJob = (jobId: string) => {
    setSelectedJobId(jobId);
    setActivePanel('job_detail');
  };

  const handleBack = () => {
    setActivePanel('jobs');
  };

  return (
    <ConfigProvider>
      <AdaptivityProvider>
        <AppRoot>
          <Epic
            activeStory={activeStory}
            tabbar={
              <Tabbar>
                <TabbarItem
                  onClick={() => {
                    setActiveStory('jobs');
                    setActivePanel('jobs');
                  }}
                  selected={activeStory === 'jobs'}
                  label="Задачи"
                >
                  <Icon28NewsfeedOutline />
                </TabbarItem>
                <TabbarItem
                  onClick={() => {
                    setActiveStory('balance');
                    setActivePanel('balance');
                  }}
                  selected={activeStory === 'balance'}
                  label="Баланс"
                >
                  <Icon28WalletOutline />
                </TabbarItem>
              </Tabbar>
            }
          >
            <View id="jobs" activePanel={activePanel}>
              <Panel id="jobs">
                <JobsPanel
                  onNewJob={handleOpenNewJob}
                  onViewJob={handleViewJob}
                />
              </Panel>
              <Panel id="new_job">
                <NewJobPanel
                  onBack={handleBack}
                  onJobCreated={handleJobCreated}
                />
              </Panel>
              <Panel id="job_detail">
                <JobDetailPanel
                  jobId={selectedJobId}
                  onBack={handleBack}
                />
              </Panel>
            </View>

            <View id="balance" activePanel="balance">
              <Panel id="balance">
                <BalancePanel />
              </Panel>
            </View>
          </Epic>
        </AppRoot>
      </AdaptivityProvider>
    </ConfigProvider>
  );
}
