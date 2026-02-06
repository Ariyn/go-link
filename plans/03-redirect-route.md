# 계획 03 - 커스텀 리다이렉트 라우트

Status: done

## 목표
- GET /:slug 구현
- slug 검증 및 링크 레코드 조회

## 작업
- URL 경로에서 slug 추출
- 소문자로 정규화
- 허용 문자 [a-z0-9-_] 검증; 잘못된 값은 거부
- slug가 일치하고 enabled가 true인 `links` 조회
- 없으면 404 반환
