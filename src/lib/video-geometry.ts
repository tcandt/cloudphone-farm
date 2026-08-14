export interface VideoContentGeometry {
  elementWidth: number;
  elementHeight: number;
  videoWidth: number;
  videoHeight: number;
  contentWidth: number;
  contentHeight: number;
  offsetX: number;
  offsetY: number;
  scale: number;
  orientation: 'portrait' | 'landscape';
  revision: number;
}

export interface NormalizedPoint {
  x: number; // 0..1
  y: number; // 0..1
}

export interface PointerScreenPosition {
  clientX: number;
  clientY: number;
}

export function computeVideoGeometry(
  videoElement: HTMLVideoElement | HTMLCanvasElement | { clientWidth?: number; clientHeight?: number; videoWidth?: number; videoHeight?: number; width?: number; height?: number },
  revision = 0
): VideoContentGeometry {
  const rawElementWidth = 'clientWidth' in videoElement && typeof videoElement.clientWidth === 'number' ? videoElement.clientWidth : 0;
  const rawElementHeight = 'clientHeight' in videoElement && typeof videoElement.clientHeight === 'number' ? videoElement.clientHeight : 0;

  const rawVideoWidth = 'videoWidth' in videoElement && typeof videoElement.videoWidth === 'number' ? videoElement.videoWidth : 0;
  const rawVideoHeight = 'videoHeight' in videoElement && typeof videoElement.videoHeight === 'number' ? videoElement.videoHeight : 0;

  const attrWidth = 'width' in videoElement && typeof videoElement.width === 'number' ? videoElement.width : 0;
  const attrHeight = 'height' in videoElement && typeof videoElement.height === 'number' ? videoElement.height : 0;

  const elementWidth = rawElementWidth > 0 ? rawElementWidth : (attrWidth > 0 ? attrWidth : 360);
  const elementHeight = rawElementHeight > 0 ? rawElementHeight : (attrHeight > 0 ? attrHeight : 640);

  const videoWidth = rawVideoWidth > 0 ? rawVideoWidth : elementWidth;
  const videoHeight = rawVideoHeight > 0 ? rawVideoHeight : elementHeight;

  const scale = Math.min(elementWidth / videoWidth, elementHeight / videoHeight);
  const contentWidth = videoWidth * scale;
  const contentHeight = videoHeight * scale;

  const offsetX = (elementWidth - contentWidth) / 2;
  const offsetY = (elementHeight - contentHeight) / 2;

  const orientation: 'portrait' | 'landscape' = videoWidth >= videoHeight ? 'landscape' : 'portrait';

  return {
    elementWidth,
    elementHeight,
    videoWidth,
    videoHeight,
    contentWidth,
    contentHeight,
    offsetX,
    offsetY,
    scale,
    orientation,
    revision,
  };
}

export function mapPointerToNormalizedCoordinates(
  position: PointerScreenPosition,
  videoBoundingRect: DOMRect | { left: number; top: number; width: number; height: number },
  geometry: VideoContentGeometry
): NormalizedPoint | null {
  const localX = position.clientX - videoBoundingRect.left - geometry.offsetX;
  const localY = position.clientY - videoBoundingRect.top - geometry.offsetY;

  // Reject pointer events falling inside black letterbox/pillarbox bars
  if (localX < 0 || localY < 0 || localX > geometry.contentWidth || localY > geometry.contentHeight) {
    return null;
  }

  const x = Math.max(0, Math.min(1, localX / geometry.contentWidth));
  const y = Math.max(0, Math.min(1, localY / geometry.contentHeight));

  return { x, y };
}
