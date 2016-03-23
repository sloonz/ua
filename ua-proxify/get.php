<?php
define("HMAC_KEY", ENTER_HMAC_KEY); // Generate the key with openssl rand -base64 32
header("Content-type: text/plain");

if(!isset($_GET['url'])) {
	http_response_code(400);
	exit;
}

$url = $_GET['url'];

if(!preg_match("/^https?:\/\//", $url)) {
	http_response_code(400);
	exit;
}

if(HMAC_KEY) {
	$sig = hash_hmac("sha256", $url, base64_decode(HMAC_KEY));
	if(!isset($_GET["sig"]) || !hash_equals($_GET["sig"], $sig)) {
		http_response_code(401);
		exit;
	}
}

$headers = [];
if(isset($_SERVER["HTTP_IF_MODIFIED_SINCE"])) {
	$headers[] = "If-Modified-Since: {$_SERVER["HTTP_IF_MODIFIED_SINCE"]}";
}
if(isset($_SERVER["HTTP_USER_AGENT"])) {
	$headers[] = "User-Agent: {$_SERVER["HTTP_USER_AGENT"]}";
}

$ctx = stream_context_create(["http" => [
	"follow_location" => 1,
	"header" => $headers
]]);

$stream = fopen($_GET['url'], 'r', false, $ctx);
foreach(stream_get_meta_data($stream)["wrapper_data"] as $fullHeader) {
	if(strpos($fullHeader, ':') === false) {
		list($httpVersion, $code, $reason) = explode(" ", $fullHeader, 3);
		header($fullHeader);
		http_response_code($code);
		continue;
	}

	list($header, $val) = explode(":", $fullHeader, 2);
	$header = strtolower(trim($header));
	$val = trim($val);

	if(in_array($header, ["content-type", "last-modified", "expires", "cache-control", "content-length"])) {
		header($fullHeader);
	}
}

stream_copy_to_stream($stream, fopen("php://output", "a+"));
