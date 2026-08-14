import { describe, it, expect, beforeEach } from 'vitest';
import { PointerGestureRecognizer } from '../lib/pointer-gesture-engine';
import { VideoContentGeometry } from '../lib/video-geometry';

describe('PointerGestureRecognizer Pipeline Suite', () => {
  let recognizer: PointerGestureRecognizer;
  let portraitGeom: VideoContentGeometry;

  const mockRect: DOMRect = {
    left: 100,
    top: 50,
    width: 360,
    height: 640,
    right: 460,
    bottom: 690,
    x: 100,
    y: 50,
    toJSON: () => {},
  };

  beforeEach(() => {
    recognizer = new PointerGestureRecognizer();
    portraitGeom = {
      elementWidth: 360,
      elementHeight: 640,
      videoWidth: 720,
      videoHeight: 1280,
      contentWidth: 360,
      contentHeight: 640,
      offsetX: 0,
      offsetY: 0,
      scale: 0.5,
      orientation: 'portrait',
      revision: 1,
    };
  });

  it('recognizes short movement as touch gesture', () => {
    const accepted = recognizer.onPointerDown(1, { clientX: 280, clientY: 370 }, mockRect, portraitGeom);
    expect(accepted).toBe(true);

    const gesture = recognizer.onPointerUp(1, { clientX: 282, clientY: 371 }, mockRect, portraitGeom);
    expect(gesture).not.toBeNull();
    expect(gesture?.type).toBe('gesture.touch');
    if (gesture?.type === 'gesture.touch') {
      expect(gesture.payload.x).toBeCloseTo(0.5, 2);
      expect(gesture.payload.y).toBeCloseTo(0.5, 2);
      expect(gesture.payload.coordinateSpace).toBe('normalized_display_v1');
      expect(gesture.payload.orientation).toBe('portrait');
    }
  });

  it('recognizes long drag movement as swipe gesture', () => {
    recognizer.onPointerDown(1, { clientX: 180, clientY: 200 }, mockRect, portraitGeom);
    recognizer.onPointerMove(1, { clientX: 180, clientY: 450 }, mockRect, portraitGeom);

    const gesture = recognizer.onPointerUp(1, { clientX: 180, clientY: 500 }, mockRect, portraitGeom);
    expect(gesture).not.toBeNull();
    expect(gesture?.type).toBe('gesture.swipe');
    if (gesture?.type === 'gesture.swipe') {
      expect(gesture.payload.startX).toBeCloseTo(0.222, 2);
      expect(gesture.payload.startY).toBeCloseTo(0.234, 2);
      expect(gesture.payload.endX).toBeCloseTo(0.222, 2);
      expect(gesture.payload.endY).toBeCloseTo(0.703, 2);
      expect(gesture.payload.durationMs).toBeGreaterThanOrEqual(50);
    }
  });

  it('returns null gesture on pointercancel', () => {
    recognizer.onPointerDown(1, { clientX: 280, clientY: 370 }, mockRect, portraitGeom);
    recognizer.cancelCurrentGesture();

    const gesture = recognizer.onPointerUp(1, { clientX: 280, clientY: 370 }, mockRect, portraitGeom);
    expect(gesture).toBeNull();
  });

  it('rejects pointerdown falling inside black letterbox bar', () => {
    const pillarboxGeom: VideoContentGeometry = {
      ...portraitGeom,
      contentWidth: 180,
      offsetX: 90, // Left black bar 0..90px
    };

    // Click at clientX = 140 (localX = 140 - 100 - 90 = -50 -> Outside)
    const accepted = recognizer.onPointerDown(1, { clientX: 140, clientY: 200 }, mockRect, pillarboxGeom);
    expect(accepted).toBe(false);
  });

  it('cancels gesture if geometry revision changes during drag', () => {
    recognizer.onPointerDown(1, { clientX: 280, clientY: 370 }, mockRect, portraitGeom);

    const rotatedGeom: VideoContentGeometry = {
      ...portraitGeom,
      orientation: 'landscape',
      revision: 2,
    };

    recognizer.onPointerMove(1, { clientX: 300, clientY: 400 }, mockRect, rotatedGeom);
    const gesture = recognizer.onPointerUp(1, { clientX: 300, clientY: 400 }, mockRect, rotatedGeom);

    expect(gesture).toBeNull();
  });
});
