import { describe, it, expect } from 'vitest';
import { computeVideoGeometry, mapPointerToNormalizedCoordinates } from '../lib/video-geometry';

describe('Video Geometry Engine', () => {
  it('computes exact content rectangle for 9:16 portrait video inside 360x640 container', () => {
    const video = { clientWidth: 360, clientHeight: 640, videoWidth: 720, videoHeight: 1280 };
    const geom = computeVideoGeometry(video);

    expect(geom).not.toBeNull();
    if (!geom) return;

    expect(geom.contentWidth).toBe(360);
    expect(geom.contentHeight).toBe(640);
    expect(geom.offsetX).toBe(0);
    expect(geom.offsetY).toBe(0);
    expect(geom.orientation).toBe('portrait');
  });

  it('fails closed when video intrinsic dimensions are 0x0', () => {
    const uninitializedVideo = { clientWidth: 360, clientHeight: 640, videoWidth: 0, videoHeight: 0 };
    const geom = computeVideoGeometry(uninitializedVideo);

    expect(geom).toBeNull();
  });

  it('computes pillarbox offsets for 9:16 video in wider 400x400 container', () => {
    const video = { clientWidth: 400, clientHeight: 400, videoWidth: 720, videoHeight: 1280 };
    const geom = computeVideoGeometry(video);

    expect(geom).not.toBeNull();
    if (!geom) return;

    expect(geom.scale).toBeCloseTo(400 / 1280, 4); // 0.3125
    expect(geom.contentWidth).toBeCloseTo(225, 1);
    expect(geom.contentHeight).toBe(400);
    expect(geom.offsetX).toBeCloseTo(87.5, 1);
    expect(geom.offsetY).toBe(0);
  });

  it('rejects click inside left/right pillarbox black bars', () => {
    const video = { clientWidth: 400, clientHeight: 400, videoWidth: 720, videoHeight: 1280 };
    const geom = computeVideoGeometry(video)!;

    const rect: DOMRect = {
      left: 100,
      top: 100,
      width: 400,
      height: 400,
      right: 500,
      bottom: 500,
      x: 100,
      y: 100,
      toJSON: () => {},
    };

    // Click at x=120 (localX = 20, which is < offsetX 87.5) -> Outside
    const point1 = mapPointerToNormalizedCoordinates({ clientX: 120, clientY: 200 }, rect, geom);
    expect(point1).toBeNull();

    // Click at center x=300, y=300 -> Inside (localX = 200 - 87.5 = 112.5)
    const point2 = mapPointerToNormalizedCoordinates({ clientX: 300, clientY: 300 }, rect, geom);
    expect(point2).not.toBeNull();
    expect(point2?.x).toBeGreaterThan(0);
    expect(point2?.x).toBeLessThan(1);
  });

  it('maps center pointer correctly to 0.5, 0.5', () => {
    const video = { clientWidth: 360, clientHeight: 640, videoWidth: 720, videoHeight: 1280 };
    const geom = computeVideoGeometry(video)!;

    const rect: DOMRect = {
      left: 0,
      top: 0,
      width: 360,
      height: 640,
      right: 360,
      bottom: 640,
      x: 0,
      y: 0,
      toJSON: () => {},
    };

    const point = mapPointerToNormalizedCoordinates({ clientX: 180, clientY: 320 }, rect, geom);
    expect(point).not.toBeNull();
    expect(point?.x).toBeCloseTo(0.5, 2);
    expect(point?.y).toBeCloseTo(0.5, 2);
  });
});
