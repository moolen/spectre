import { TimeRangeInfo } from '../services/apiTypes';
import { TimeRange } from '../types';

export function buildWatcherDisabledInitialRange(bounds: TimeRangeInfo | null | undefined): TimeRange | null {
  if (!bounds) {
    return null;
  }

  const { earliest, latest } = bounds;
  if (earliest <= 0 || latest <= 0 || latest < earliest) {
    return null;
  }

  if (earliest === latest) {
    return {
      start: new Date((earliest - 1) * 1000),
      end: new Date((latest + 1) * 1000),
    };
  }

  return {
    start: new Date(earliest * 1000),
    end: new Date(latest * 1000),
  };
}
