<?php
$brand = json_decode(file_get_contents(__DIR__ . '/brand.json'), true, 512, JSON_THROW_ON_ERROR);
$config['product_name'] = $brand['name'] . ' — почта поддержки';
$config['skin_logo'] = 'skins/epifera/images/brand.svg?v=__EPIFERA_UI_REVISION__';
$config['dont_override'] = ['skin'];
$config['support_url'] = 'https://epifera.com/';
$config['use_https'] = true;
$config['session_lifetime'] = 30;
$config['des_key'] = getenv('EPIFERA_ROUNDCUBE_DES_KEY');
$config['smtp_helo_host'] = 'mail.epifera.com';
$config['imap_conn_options'] = [
    'ssl' => ['verify_peer' => true, 'verify_peer_name' => true, 'allow_self_signed' => false],
];
$config['smtp_conn_options'] = $config['imap_conn_options'];
$config['imap_debug'] = false;
$config['smtp_debug'] = false;
