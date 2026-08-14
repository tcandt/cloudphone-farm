import { describe, it, expect } from 'vitest';
import { computeVideoGeometry, mapPointerToNormalizedCoordinates } from '../lib/video-geometry';

describe('Video Geometry Engine', () => {
  it('computes exact content geometry for portrait video in 9:16 container', () => {
    const video = {
      clientWidth: 360,
      clientHeight: 640,
      videoWidth: 720,
      videoHeight: 1280,
    };

    const geom = computeVideoGeometry(video, 1);
    expect(geom).not.toBeNull();
    if (!geom) return;

    expect(geom.contentWidth).toBe(360);
    expect(geom.contentHeight).toBe(640);
    expect(geom.offsetX).toBe(0);
    expect(geom.offsetY).toBe(0);
    expect(geom.orientation).toBe('portrait');
  });

  it('computes pillarbox offsets for portrait video in square container', () => {
    const video = {
      clientWidth: 400,
      clientHeight: 400,
      videoWidth: 720,
      videoHeight: 1280,
    };

    const geom = computeVideoGeometry(video, 2);
    expect(geom).not.toBeNull();
    if (!geom) return;

    // scale = 400 / 1280 = 0.3125
    // contentWidth = 720 * 0.3125 = 225
    // contentHeight = 400
    // offsetX = (400 - 225) / 2 = 87.5
    expect(geom.contentWidth).toBe(225);
    expect(geom.contentHeight).toBe(400);
    expect(geom.offsetX).toBe(87.5);
    expect(geom.offsetY).toBe(0);
  });

  it('rejects click inside left/right pillarbox black bars', () => {
    const video = {
      clientWidth: 400,
      clientHeight: 400,
      videoWidth: 720,
      videoHeight: 1280,
    };
    const geom = computeVideoGeometry(video, 1)!;
    const rect = { left: 100, top: 100, width: 400, height: 400 };

    // Click at x=120 (localX = 20, which is < offsetX 87.5) -> Outside
    const point1 = mapPointerToNormalizedCoordinates({ clientX: 120, clientY: 300 }, rect, geom);
    expect(point1).toBeNull();

    // Click at center x=300, y=300 -> Inside (localX = 200 - 87.5 = 112.5, normX = 112.5 / 225 = 0.5)
    const point2 = mapPointerToNormalizedCoordinates({ clientX: 300, clientY: 300 }, rect, geom);
    expect(point2).not.toBeNull();
    expect(point2?.x).toBeCloseTo(0.5, 2);
    expect(point2?.y).toBeCloseTo(0.5, 2);
  });

  it('correctly maps top-left and bottom-right bounds inside content area', () => {
    const video = {
      clientWidth: 360,
      clientHeight: 640,
      videoWidth: 720,
      videoHeight: 1280,
    };
    const geom = computeVideoGeometry(video, 1)!;
    const rect = { left: 0, top: 0, width: 360, height: 640 };

    const topLeft = mapPointerToNormalizedCoordinates({ clientX: 0, clientY: 0 }, rect, geom);
    expect(topLeft).toEqual({ x: 0, y: 0 });

    const bottomRight = mapPointerToNormalizedCoordinates({ clientX: 360, clientY: 640 }, rect, geom);
    expect(bottomRight).toEqual({ x: 1, y: 1 });
  });

  it('handles landscape orientation video in portrait container with letterbox', () => {
    const video = {
      clientWidth: 360,
      clientHeight: 640,
      videoWidth: 1280,
      videoHeight: 720,
    };
    const geom = computeVideoGeometry(video, 3)!;
    expect(geom.orientation).toBe('landscape');
    expect(geom.contentWidth).toBe(360);
    // scale = 360 / 1280 = 0.28125
    // contentHeight = 720 * 0.28125 = 202.5
    // offsetY = (640 - 202.5) / 2 = 218.75
    expect(geom.offsetY).toBeCloseTo(218.75, 2);
  });
});
