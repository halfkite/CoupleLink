const path = require('node:path');
const root=process.env.COUPLELINK_TEST_POSTGRES_ROOT;
if(!root)throw Error('Use prepare-test-postgres.ps1');
const {Client}=require(path.join(root,'.tools/pg/node_modules/pg'));
const client=new Client({host:'127.0.0.1',port:55433,user:'couplelink',password:process.env.PGPASSWORD,database:'postgres'});
(async()=>{await client.connect();for(const name of ['couplelink_test','couplelink_dev']){
 const result=await client.query('SELECT 1 FROM pg_database WHERE datname=$1',[name]);
 if(!result.rowCount)await client.query(`CREATE DATABASE ${name}`);
}await client.end();console.log('Isolated test and development databases prepared.');})().catch(error=>{console.error(error.message);process.exitCode=1;});
