// Package wechat parses public WeChat official-account article pages
// (mp.weixin.qq.com) without any login. It is used to resolve article links
// to their account (`biz` fakeid, name, avatar) and to extract article HTML.
//
// The extraction logic is a Go port of the corresponding public-page flow in
// rachelos/we-mp-rss (https://github.com/rachelos/we-mp-rss), which is
// distributed under the MIT License (core/article_content.py).
//
// The page structure is unofficial and may change without notice. The
// package is not affiliated with Tencent.
package wechat
