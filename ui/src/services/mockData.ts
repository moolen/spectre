import { K8sResource, ResourceStatus, K8sEvent, ResourceStatusSegment } from '../types';
import { MOCK_NAMESPACES, MOCK_KINDS } from '../constants';

const START_TIME = new Date();
START_TIME.setHours(START_TIME.getHours() - 2); // Start 2 hours ago

const END_TIME = new Date();

function randomInt(min: number, max: number) {
  return Math.floor(Math.random() * (max - min + 1) + min);
}

function randomItem<T>(arr: T[]): T {
  return arr[randomInt(0, arr.length - 1)];
}

// Generate a realistic status timeline for a resource
function generateStatusSegments(startTime: Date, endTime: Date): ResourceStatusSegment[] {
  const segments: ResourceStatusSegment[] = [];
  let currentTime = new Date(startTime.getTime() + randomInt(0, 1000 * 60 * 10)); // Offset start
  
  // Initial status
  let currentStatus = Math.random() > 0.8 ? ResourceStatus.Warning : ResourceStatus.Ready;
  
  // Initial Config
  let currentConfig: Record<string, any> = {
    replicas: randomInt(1, 5),
    image: `my-app:${randomInt(1, 5)}.0.0`,
    memoryRequest: '128Mi',
    cpuLimit: '500m',
    env: 'production',
    logLevel: 'INFO'
  };

  while (currentTime < endTime) {
    const duration = randomInt(1000 * 60 * 2, 1000 * 60 * 30); // 2 to 30 mins
    const nextTime = new Date(currentTime.getTime() + duration);
    const end = nextTime > endTime ? endTime : nextTime;

    segments.push({
      start: new Date(currentTime),
      end: new Date(end),
      status: currentStatus,
      message: `Status transitioned to ${currentStatus}`,
      config: { ...currentConfig } // Snapshot config
    });

    currentTime = nextTime;
    
    // Change status randomly
    const rand = Math.random();
    if (currentStatus === ResourceStatus.Ready) {
      if (rand > 0.9) {
        currentStatus = ResourceStatus.Error;
      } else if (rand > 0.7) {
        currentStatus = ResourceStatus.Warning;
      }
    } else {
      // High chance to recover
      if (rand > 0.3) currentStatus = ResourceStatus.Ready;
    }

    // Mutate config for next segment occasionally
    if (Math.random() > 0.6) {
      if (Math.random() > 0.5) currentConfig.replicas = randomInt(1, 10);
      else if (Math.random() > 0.5) currentConfig.image = `my-app:${randomInt(6, 10)}.0.0`;
      else if (Math.random() > 0.8) currentConfig.logLevel = 'DEBUG';
      else if (Math.random() > 0.8) delete currentConfig.cpuLimit; // Simulate removal
    }
  }

  return segments;
}

function generateEvents(segments: ResourceStatusSegment[]): K8sEvent[] {
  const events: K8sEvent[] = [];
  
  segments.forEach(seg => {
    // Event at start of segment (transition)
    events.push({
      id: Math.random().toString(36).substr(2, 9),
      timestamp: seg.start,
      verb: 'patch',
      user: 'system:controller-manager',
      message: seg.message || 'Status update',
      details: 'Audit ID: ' + Math.random().toString(16).substr(2)
    });

    // Random noise events
    if (Math.random() > 0.7) {
      const noiseTime = new Date(seg.start.getTime() + (seg.end.getTime() - seg.start.getTime()) / 2);
      events.push({
        id: Math.random().toString(36).substr(2, 9),
        timestamp: noiseTime,
        verb: 'get',
        user: 'admin-user',
        message: 'Resource accessed',
      });
    }
  });

  return events;
}

export const generateMockData = (count: number = 20): K8sResource[] => {
  const resources: K8sResource[] = [];

  for (let i = 0; i < count; i++) {
    const kind = randomItem(MOCK_KINDS);
    const namespace = randomItem(MOCK_NAMESPACES);
    const name = `${kind.toLowerCase()}-${randomItem(['app', 'db', 'cache', 'worker', 'auth'])}-${randomInt(1000, 9999)}`;
    const segments = generateStatusSegments(START_TIME, END_TIME);

    resources.push({
      id: Math.random().toString(36).substr(2, 9),
      group: 'apps', // Simplified
      version: 'v1',
      kind,
      namespace,
      name,
      statusSegments: segments,
      events: generateEvents(segments)
    });
  }

  return resources;
};