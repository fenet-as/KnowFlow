import { Citation } from '../api/client';

export type TextSegment =
  | { type: 'text'; text: string }
  | { type: 'citation'; marker: number; citation: Citation };

/**
 * Splits an answer string containing [n] markers into plain text segments
 * and matched citation chip objects.
 * Unmatched [n] markers are kept as plain text so no content is lost.
 */
export function parseAnswerWithCitations(
  answer: string,
  citations: Citation[] = [],
): TextSegment[] {
  if (!answer) {
    return [];
  }

  const citationMap = new Map<number, Citation>();
  for (const c of citations) {
    citationMap.set(c.marker, c);
  }

  const segments: TextSegment[] = [];
  // Regex matches [n] where n is 1 or 2 digits
  const regex = /\[(\d{1,2})\]/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(answer)) !== null) {
    const matchIndex = match.index;
    const markerNum = parseInt(match[1], 10);

    // Append preceding plain text
    if (matchIndex > lastIndex) {
      segments.push({
        type: 'text',
        text: answer.slice(lastIndex, matchIndex),
      });
    }

    const matchedCitation = citationMap.get(markerNum);
    if (matchedCitation) {
      segments.push({
        type: 'citation',
        marker: markerNum,
        citation: matchedCitation,
      });
    } else {
      // If marker has no citation metadata, preserve literal text
      segments.push({
        type: 'text',
        text: match[0],
      });
    }

    lastIndex = matchIndex + match[0].length;
  }

  // Append remaining text after last match
  if (lastIndex < answer.length) {
    segments.push({
      type: 'text',
      text: answer.slice(lastIndex),
    });
  }

  return segments;
}

/**
 * Formats a file size in bytes to a human-readable string (KB, MB).
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

/**
 * Formats an ISO date string into a localized readable date/time.
 */
export function formatDate(isoString: string): string {
  try {
    const d = new Date(isoString);
    return d.toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return isoString;
  }
}

