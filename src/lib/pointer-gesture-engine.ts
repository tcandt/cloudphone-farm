import {
  NormalizedPoint,
  PointerScreenPosition,
  VideoContentGeometry,
  mapPointerToNormalizedCoordinates,
} from './video-geometry';

export interface TouchGesturePayload {
  x: number;
  y: number;
  coordinateSpace: 'normalized_display_v1';
  orientation: 'portrait' | 'landscape';
}

export interface SwipeGesturePayload {
  startX: number;
  startY: number;
  endX: number;
  endY: number;
  durationMs: number;
  coordinateSpace: 'normalized_display_v1';
  orientation: 'portrait' | 'landscape';
}

export type DispatchedGesture =
  | { type: 'gesture.touch'; payload: TouchGesturePayload }
  | { type: 'gesture.swipe'; payload: SwipeGesturePayload };

export interface ActivePointerGestureState {
  pointerId: number;
  startedAt: number;
  startPoint: NormalizedPoint;
  lastPoint: NormalizedPoint;
  geometryRevision: number;
  orientation: 'portrait' | 'landscape';
}

export class PointerGestureRecognizer {
  private activeGesture: ActivePointerGestureState | null = null;
  private minSwipeDistanceNormalized = 0.04; // Minimum movement (4% of display) to trigger swipe vs touch

  public onPointerDown(
    pointerId: number,
    position: PointerScreenPosition,
    videoBoundingRect: DOMRect | { left: number; top: number; width: number; height: number },
    geometry: VideoContentGeometry
  ): boolean {
    const point = mapPointerToNormalizedCoordinates(position, videoBoundingRect, geometry);
    if (!point) {
      // Pointer down outside video content area (in letterbox bar)
      this.cancelCurrentGesture();
      return false;
    }

    this.activeGesture = {
      pointerId,
      startedAt: Date.now(),
      startPoint: point,
      lastPoint: point,
      geometryRevision: geometry.revision,
      orientation: geometry.orientation,
    };
    return true;
  }

  public onPointerMove(
    pointerId: number,
    position: PointerScreenPosition,
    videoBoundingRect: DOMRect | { left: number; top: number; width: number; height: number },
    geometry: VideoContentGeometry
  ): void {
    if (!this.activeGesture || this.activeGesture.pointerId !== pointerId) {
      return;
    }

    // Cancel gesture if video geometry or orientation changed mid-drag
    if (this.activeGesture.geometryRevision !== geometry.revision || this.activeGesture.orientation !== geometry.orientation) {
      this.cancelCurrentGesture();
      return;
    }

    const point = mapPointerToNormalizedCoordinates(position, videoBoundingRect, geometry);
    if (point) {
      this.activeGesture.lastPoint = point;
    }
  }

  public onPointerUp(
    pointerId: number,
    position: PointerScreenPosition,
    videoBoundingRect: DOMRect | { left: number; top: number; width: number; height: number },
    geometry: VideoContentGeometry
  ): DispatchedGesture | null {
    if (!this.activeGesture || this.activeGesture.pointerId !== pointerId) {
      return null;
    }

    const currentGesture = this.activeGesture;
    this.activeGesture = null;

    // Reject gesture if geometry changed mid-gesture
    if (currentGesture.geometryRevision !== geometry.revision || currentGesture.orientation !== geometry.orientation) {
      return null;
    }

    const endPoint = mapPointerToNormalizedCoordinates(position, videoBoundingRect, geometry) || currentGesture.lastPoint;
    const endedAt = Date.now();
    const durationMs = Math.max(50, Math.min(5000, endedAt - currentGesture.startedAt));

    const dx = endPoint.x - currentGesture.startPoint.x;
    const dy = endPoint.y - currentGesture.startPoint.y;
    const distance = Math.sqrt(dx * dx + dy * dy);

    if (distance < this.minSwipeDistanceNormalized) {
      return {
        type: 'gesture.touch',
        payload: {
          x: currentGesture.startPoint.x,
          y: currentGesture.startPoint.y,
          coordinateSpace: 'normalized_display_v1',
          orientation: currentGesture.orientation,
        },
      };
    } else {
      return {
        type: 'gesture.swipe',
        payload: {
          startX: currentGesture.startPoint.x,
          startY: currentGesture.startPoint.y,
          endX: endPoint.x,
          endY: endPoint.y,
          durationMs,
          coordinateSpace: 'normalized_display_v1',
          orientation: currentGesture.orientation,
        },
      };
    }
  }

  public cancelCurrentGesture(): void {
    this.activeGesture = null;
  }
}
